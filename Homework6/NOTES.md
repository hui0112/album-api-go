# Homework 6 - Implementation Notes

## Current State: Part 3 (ALB + Auto Scaling)

---

## What Was Done

### 1. Copied HW5 project structure
- Copied `Homework5/CS6650_2b_demo/` files into `Homework6/`
- Kept: `src/Dockerfile`, `src/go.mod`, `src/go.sum`, `terraform/provider.tf`, and all terraform modules (`ecr`, `ecs`, `logging`, `network`)
- Did not copy: `api.yaml`, `README.MD` (HW5-specific)

### 2. Rewrote Go service (`src/main.go`)

**Why rewrite instead of modify:** HW5 was a CRUD API (GET/POST by product ID). HW6 is a search service — completely different logic.

**Key design decisions:**

| Decision | What | Why |
|----------|------|-----|
| `sync.Map` | Thread-safe map for product storage | Regular Go maps panic on concurrent read/write. sync.Map is optimized for read-heavy workloads like search |
| 100k products | Generate at startup, store in memory | Simulates realistic memory footprint; memory stays flat during load testing |
| Check exactly 100 | Counter increments for EVERY product, not just matches | Simulates fixed-cost computation (like AI inference). Makes CPU the bottleneck, which is the whole point of the scaling exercise |
| Max 20 results | Cap results array at 20, but keep counting matches | total_found can exceed 20 (shows how many matched), but response payload stays small |
| Named handlers | `healthCheck()`, `handleSearch()` as separate functions | Matches HW5 pattern (`getProduct`, `addProductDetails`). Cleaner than inline anonymous functions — easier to read, test, and reuse |

**Data generation pattern:**
```
brands[i % 8]     → Alpha, Beta, Gamma, Delta, Epsilon, Zeta, Eta, Theta
categories[i % 8] → Electronics, Books, Home, Sports, Clothing, Toys, Food, Health
```
Using modulo rotation ensures every search term gets roughly equal hits across the dataset.

### 3. Rewrote Locust load test (`locustfile.py`)

**Why only FastHttpUser:** HW5 had both HttpUser and FastHttpUser for comparison. For load testing we only need the faster one — FastHttpUser uses `geventhttpclient` which has lower overhead per request.

**Why `wait_time = between(0.01, 0.05)`:** Near-zero wait simulates maximum load per user. With 5 users that's ~100-500 requests/sec depending on response time. With 20 users, enough to saturate 0.25 vCPU.

### 4. Terraform (Part 2 only)

**What changed from HW5:**
- `variables.tf`: `service_name` → `"CS6650HW6"` (avoid naming conflicts with HW5 resources)
- Everything else is identical to HW5's terraform structure

**What was NOT added (saved for Part 3):**
- No ALB module
- No auto scaling
- No `enable_alb` conditionals
- No `vpc_id` output from network module

This keeps the code simple and debuggable. Part 3 will be layered on top.

---

## How to Deploy (Part 2)

```bash
cd Homework6/terraform
terraform init
terraform apply

# Find the ECS task public IP in AWS Console:
# ECS > Clusters > CS6650HW6-cluster > Tasks > click task > Public IP

# Test locally first
curl http://<ECS_PUBLIC_IP>:8080/health
curl http://<ECS_PUBLIC_IP>:8080/products/search?q=Alpha
```

## How to Load Test

```bash
cd Homework6

# Test 1 — Baseline: 5 users, 2 minutes
locust -f locustfile.py --host=http://<ECS_PUBLIC_IP>:8080 -u 5 -r 5 -t 2m --headless

# Test 2 — Breaking point: 20 users, 3 minutes
locust -f locustfile.py --host=http://<ECS_PUBLIC_IP>:8080 -u 20 -r 20 -t 3m --headless
```

**What to watch in CloudWatch:** ECS > Your Service > Metrics tab
- CPU should be ~60% with 5 users, near 100% with 20 users
- Memory should stay flat (products loaded once at startup)

---

## File Summary (Part 2)

| File | Status | Description |
|------|--------|-------------|
| `src/main.go` | Rewritten | Product search service with sync.Map |
| `src/Dockerfile` | Unchanged from HW5 | Multi-stage Go build |
| `src/go.mod`, `src/go.sum` | Unchanged from HW5 | Gin framework dependency |
| `locustfile.py` | Rewritten | FastHttpUser search load test |
| `terraform/main.tf` | Simplified from HW5 | Same 4 modules, updated service name |
| `terraform/variables.tf` | Modified | service_name = CS6650HW6 |
| `terraform/outputs.tf` | Same as HW5 | Cluster name + service name |
| `terraform/provider.tf` | Unchanged from HW5 | AWS + Docker providers |
| `terraform/modules/ecs/*` | Unchanged from HW5 | 256 CPU, 512 MB, 1 instance |
| `terraform/modules/network/*` | Unchanged from HW5 | Default VPC + security group |
| `terraform/modules/ecr/*` | Unchanged from HW5 | ECR repository |
| `terraform/modules/logging/*` | Unchanged from HW5 | CloudWatch log group |

---

## Key Concepts to Understand

### Why sync.Map instead of regular map + mutex?
- Go's regular `map` is NOT thread-safe — concurrent writes cause a runtime panic
- `sync.Mutex` wraps a map but blocks all readers during any write
- `sync.RWMutex` is better (multiple readers, exclusive writers) but still has lock contention
- `sync.Map` is lock-free for reads in the common case — ideal for our read-only-after-startup workload

### Why check exactly 100 products?
The assignment simulates **fixed-cost computation** — like running an ML model where every request takes the same amount of work regardless of input. This makes CPU the bottleneck (not I/O or network), which is exactly what auto scaling is designed to solve.

---

## Part 3: ALB + Auto Scaling

### What Was Added

#### 1. New `modules/alb/` module
- **ALB Security Group** — allows port 80 inbound from anywhere
- **Application Load Balancer** — internet-facing, distributes traffic across ECS tasks
- **Target Group** — type `ip` (required for Fargate), health check on `/health` every 30s
- **Listener** — listens on port 80, forwards to target group on port 8080

#### 2. Modified `modules/ecs/main.tf`
- Added `load_balancer` block to ECS service — registers tasks with ALB target group
- Changed `desired_count` from 1 to 2 (matches auto scaling min)
- Added `aws_appautoscaling_target` — min=2, max=4 tasks
- Added `aws_appautoscaling_policy` — target tracking on CPU 70%, 300s cooldown both directions

#### 3. Modified `terraform/main.tf`
- Added `module "alb"` block wired to network module outputs
- Passed `target_group_arn` from ALB module to ECS module
- Changed `ecs_count` to 2

### Traffic Flow
```
User → ALB (port 80) → Target Group → ECS Task 1 (port 8080)
                                     → ECS Task 2 (port 8080)
                                     → ECS Task 3 (if scaled up)
                                     → ECS Task 4 (if scaled up)
```

### Auto Scaling Behavior
```
CPU < 70% for 300s → remove 1 task (down to min 2)
CPU > 70% for 300s → add 1 task (up to max 4)
```

### How to Deploy (Part 3)

```bash
cd Homework6/terraform
terraform init
terraform apply

# Get ALB DNS from terraform output
curl http://<ALB_DNS>/health
curl http://<ALB_DNS>/products/search?q=Alpha
```

### How to Load Test (Part 3)

```bash
cd Homework6
locust -f locustfile.py --host=http://<ALB_DNS>
# Run 20 users, ramp up 20, 3 minutes
```

**What to watch in AWS Console:**
- ECS > Service > Tasks tab (watch task count go from 2 → 3 → 4)
- ECS > Service > Metrics (CPU per instance should stay lower than Part 2)
- EC2 > Target Groups > Targets (see healthy instances)

### File Summary (Part 3 changes)

| File | Action | Description |
|------|--------|-------------|
| `terraform/modules/alb/main.tf` | Created | ALB, target group, listener, security group |
| `terraform/modules/alb/variables.tf` | Created | service_name, vpc_id, subnet_ids, container_port |
| `terraform/modules/alb/outputs.tf` | Created | alb_dns_name, target_group_arn, alb_security_group_id |
| `terraform/modules/ecs/main.tf` | Modified | Added load_balancer block + auto scaling resources |
| `terraform/modules/ecs/variables.tf` | Modified | Added target_group_arn variable |
| `terraform/main.tf` | Modified | Added ALB module, passed target_group_arn to ECS |
| `terraform/outputs.tf` | Modified | Added alb_dns_name output |
