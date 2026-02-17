# CS6650 Homework 5 - Product API

## Project Structure

```
CS6650_2b_demo/
├── src/
│   ├── main.go          # Go server — Product API (GET & POST endpoints)
│   ├── Dockerfile       # Multi-stage Docker build for the Go server
│   ├── go.mod           # Go module dependencies
│   └── go.sum           # Go dependency checksums
├── terraform/
│   ├── provider.tf      # AWS & Docker provider config
│   ├── variables.tf     # Configurable variables (region, port, etc.)
│   ├── main.tf          # Wires modules together, builds & pushes Docker image
│   ├── outputs.tf       # Outputs (cluster name, service name)
│   └── modules/
│       ├── network/     # VPC, subnets, security group
│       ├── ecr/         # Elastic Container Registry (Docker image storage)
│       ├── ecs/         # ECS Fargate cluster, task definition, service
│       └── logging/     # CloudWatch log group
├── locustfile.py        # Locust load test (HttpUser & FastHttpUser)
├── api.yaml             # OpenAPI 3.0.3 specification
└── Instructions.md      # Assignment instructions
```

## Deployment Instructions

### Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Terraform](https://developer.hashicorp.com/terraform/downloads)
- [Docker](https://docs.docker.com/get-docker/)
- AWS CLI configured with credentials

### 1. Set Up AWS Credentials

```bash
aws configure
aws configure set aws_session_token <YOUR-TEMP-SESSION-TOKEN>
```

### 2. Deploy Infrastructure

```bash
cd terraform
terraform init
terraform apply -auto-approve
```

### 3. Get the Public IP

From the `terraform/` directory:

```bash
aws ec2 describe-network-interfaces \
--network-interface-ids $(
    aws ecs describe-tasks \
    --cluster $(terraform output -raw ecs_cluster_name) \
    --tasks $(
        aws ecs list-tasks \
        --cluster $(terraform output -raw ecs_cluster_name) \
        --service-name $(terraform output -raw ecs_service_name) \
        --query 'taskArns[0]' --output text
    ) \
    --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
    --output text
) \
--query 'NetworkInterfaces[0].Association.PublicIp' \
--output text
```

### 4. Send Requests

Replace `<PUBLIC-IP>` with the IP from step 3 (or use `localhost:8080` for local testing).

See the API Examples section below for all endpoints and response codes.

### 5. Clean Up

```bash
terraform destroy -auto-approve
```

## Running Locally

```bash
cd src
go run main.go
```

Server starts on http://localhost:8080.

## API Examples

### POST /products/{productId}/details — Add product details

**204 — Success (product created):**

```bash
curl -X POST http://localhost:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "sku": "LAPTOP-001",
    "manufacturer": "Dell",
    "category_id": 10,
    "weight": 2500,
    "some_other_id": 42
  }'
```

Response: empty body, status 204.

**400 — Invalid product ID:**

```bash
curl -X POST http://localhost:8080/products/abc/details \
  -H "Content-Type: application/json" \
  -d '{"sku": "X", "manufacturer": "Y", "category_id": 1, "weight": 0, "some_other_id": 1}'
```

Response:
```json
{
  "error": "INVALID_INPUT",
  "message": "Product ID must be a positive integer"
}
```

**400 — Missing/empty required fields:**

```bash
curl -X POST http://localhost:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{"sku": "", "manufacturer": "Dell", "category_id": 10, "weight": 100, "some_other_id": 1}'
```

Response:
```json
{
  "error": "INVALID_INPUT",
  "message": "sku and manufacturer are required and cannot be empty"
}
```

**400 — Invalid JSON body:**

```bash
curl -X POST http://localhost:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d 'not json'
```

Response:
```json
{
  "error": "INVALID_INPUT",
  "message": "Invalid JSON in request body",
  "details": "invalid character 'o' in literal null (expecting 'u')"
}
```

### GET /products/{productId} — Get product by ID

**200 — Product found** (after creating it with POST):

```bash
curl http://localhost:8080/products/1
```

Response:
```json
{
  "product_id": 1,
  "sku": "LAPTOP-001",
  "manufacturer": "Dell",
  "category_id": 10,
  "weight": 2500,
  "some_other_id": 42
}
```

**404 — Product not found:**

```bash
curl http://localhost:8080/products/999
```

Response:
```json
{
  "error": "NOT_FOUND",
  "message": "Product not found"
}
```

**400 — Invalid product ID:**

```bash
curl http://localhost:8080/products/abc
```

Response:
```json
{
  "error": "INVALID_INPUT",
  "message": "Product ID must be a positive integer"
}
```

## Load Testing with Locust

### Install Locust

```bash
pip3 install locust
```

### Run Load Test

Start the Go server in one terminal, then in another:

```bash
locust
```

Open http://localhost:8089 and configure:
- **Host**: `http://localhost:8080` (or `http://<PUBLIC-IP>:8080` for AWS)
- **Number of users**: start with 10, then try 100, 500, 1000
- **Spawn rate**: 5-100

The locustfile includes both `HttpUser` and `FastHttpUser` to compare performance. Tasks are weighted 3:1 (GET:POST) to simulate realistic read-heavy e-commerce traffic.
