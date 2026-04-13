# Album Store API

Production-ready REST API for CS6650 Final Mastery Project. Implements album and photo management with DynamoDB, S3, and ECS deployment.

## 📚 Documentation

### English
- **[Quick Start Guide](docs/QUICKSTART.md)** - Get started in 5 minutes
- **[Implementation Guide](docs/IMPLEMENTATION.md)** - Deep technical dive
- **[Summary](docs/SUMMARY.md)** - Implementation overview

### 中文文档
- **[项目背景知识](docs/项目背景.md)** - 承上启下，系统讲解本项目的背景、目的、技术选型和实现细节

### Specifications
- **[API Specification](album-store.openapi.yaml)** - OpenAPI spec (root)
- **[Assignment Details](album_store.md)** - Project requirements (root)

## Architecture

- **Framework**: Gin (Go web framework)
- **Database**: DynamoDB (albums + photos tables)
- **Storage**: AWS S3 (photo files)
- **Deployment**: ECS Fargate + Application Load Balancer
- **Infrastructure**: Terraform

## Features

✅ **Album Management**
- Create/update albums (idempotent PUT)
- Retrieve album details
- List all albums with pagination

✅ **Photo Management**
- Async photo upload (202 Accepted)
- Background processing to S3
- Per-album atomic sequence numbers
- Fast deletion (< 5 seconds, parallel DynamoDB + S3)
- Photo status tracking (processing/completed/failed)

✅ **Performance Optimizations**
- Atomic sequence increments (DynamoDB conditional updates)
- Parallel delete operations
- Connection pooling (AWS SDK v2)
- Pagination for large datasets

## Quick Start

### Prerequisites

- Go 1.24+
- Docker
- Terraform 1.0+
- AWS CLI configured with credentials
- AWS account with permissions for ECS, DynamoDB, S3, ECR, IAM

### Local Development

1. **Install dependencies:**
   ```bash
   go mod download
   ```

2. **Set environment variables:**
   ```bash
   export ALBUMS_TABLE=test-albums
   export PHOTOS_TABLE=test-photos
   export S3_BUCKET=test-photos-bucket
   export AWS_REGION=us-west-2
   export PORT=8080
   ```

3. **Run locally:**
   ```bash
   go run src/*.go
   ```

4. **Test endpoints:**
   ```bash
   # Health check
   curl http://localhost:8080/health

   # Create album
   curl -X PUT http://localhost:8080/albums/a1 \
     -H "Content-Type: application/json" \
     -d '{
       "album_id": "a1",
       "title": "Vacation 2024",
       "description": "Summer trip photos",
       "owner": "user@example.com"
     }'

   # Upload photo
   curl -X POST http://localhost:8080/albums/a1/photos \
     -F "photo=@test.jpg"

   # Get photo status
   curl http://localhost:8080/albums/a1/photos/{photo_id}

   # List photos
   curl http://localhost:8080/albums/a1/photos

   # Delete photo
   curl -X DELETE http://localhost:8080/albums/a1/photos/{photo_id}
   ```

### AWS Deployment

1. **Deploy infrastructure and application:**
   ```bash
   chmod +x deploy.sh
   ./deploy.sh prod us-west-2
   ```

   This script will:
   - Create DynamoDB tables, S3 bucket, ECR repository, ECS cluster, ALB
   - Build and push Docker image to ECR
   - Deploy ECS service with 2 tasks

2. **Get ALB URL:**
   ```bash
   cd terraform
   terraform output alb_url
   ```

3. **Test deployment:**
   ```bash
   ALB_URL=$(cd terraform && terraform output -raw alb_url)
   curl http://$ALB_URL/health
   ```

### Manual Deployment Steps

If you prefer manual deployment:

1. **Create infrastructure:**
   ```bash
   cd terraform
   terraform init
   terraform apply -var="environment=prod" -var="aws_region=us-west-2"
   ```

2. **Build and push Docker image:**
   ```bash
   # Get ECR URL
   ECR_REPO=$(cd terraform && terraform output -raw ecr_repository_url)

   # Build image
   docker build -t album-store:latest .

   # Login to ECR
   aws ecr get-login-password --region us-west-2 | \
     docker login --username AWS --password-stdin $ECR_REPO

   # Push image
   docker tag album-store:latest $ECR_REPO:latest
   docker push $ECR_REPO:latest
   ```

3. **Update ECS service:**
   ```bash
   aws ecs update-service \
     --cluster prod-album-store-cluster \
     --service prod-album-store-service \
     --force-new-deployment \
     --region us-west-2
   ```

## API Endpoints

| Method | Endpoint | Description | Status Code |
|--------|----------|-------------|-------------|
| GET | `/health` | Health check | 200 |
| PUT | `/albums/:album_id` | Create/update album | 201 (new) / 200 (update) |
| GET | `/albums/:album_id` | Get album details | 200 / 404 |
| GET | `/albums` | List all albums | 200 |
| POST | `/albums/:album_id/photos` | Upload photo | 202 |
| GET | `/albums/:album_id/photos/:photo_id` | Get photo details | 200 / 404 |
| GET | `/albums/:album_id/photos` | List album photos | 200 |
| DELETE | `/albums/:album_id/photos/:photo_id` | Delete photo | 204 / 404 |

## Data Models

### Album
```json
{
  "album_id": "string",
  "title": "string",
  "description": "string",
  "owner": "string (email)"
}
```

### Photo
```json
{
  "photo_id": "string (UUID)",
  "album_id": "string",
  "seq": "integer (monotonic per album)",
  "status": "string (processing|completed|failed)",
  "url": "string (S3 URL, optional)"
}
```

## Key Implementation Details

### Atomic Sequence Numbers

Per-album sequence numbers are guaranteed to be monotonic using DynamoDB atomic operations:

```go
UpdateItem with:
  UpdateExpression: "ADD next_seq :inc"
  ReturnValues: UPDATED_NEW
```

This ensures no race conditions even with concurrent uploads.

### Async Photo Upload Flow

1. Validate album exists
2. Atomically increment sequence → get seq number
3. Generate UUID for photo_id
4. Create photo record with `status="processing"`
5. Return 202 immediately with photo_id, seq, status
6. Background goroutine uploads to S3 and updates status to "completed"

### Fast Photo Deletion

Parallel execution (< 5 seconds):
- Goroutine 1: Delete from DynamoDB
- Goroutine 2: Delete from S3

Both operations run concurrently and complete in ~1-2 seconds total.

### Idempotent Album Creation

`PUT /albums/:album_id` uses DynamoDB PutItem which overwrites existing records:
- Returns 201 if album is new (checked before PUT)
- Returns 200 if album already exists
- Preserves `next_seq` counter on updates

## Monitoring

### CloudWatch Logs
```bash
aws logs tail /ecs/prod-album-store --follow --region us-west-2
```

### ECS Service Status
```bash
aws ecs describe-services \
  --cluster prod-album-store-cluster \
  --services prod-album-store-service \
  --region us-west-2
```

### DynamoDB Metrics
```bash
aws cloudwatch get-metric-statistics \
  --namespace AWS/DynamoDB \
  --metric-name ConsumedReadCapacityUnits \
  --dimensions Name=TableName,Value=prod-albums \
  --start-time 2024-01-01T00:00:00Z \
  --end-time 2024-01-01T23:59:59Z \
  --period 3600 \
  --statistics Sum \
  --region us-west-2
```

## Testing with ChaosArena

1. **Submit to ChaosArena:**
   ```bash
   ALB_URL=$(cd terraform && terraform output -raw alb_url)

   curl -X POST http://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/submit \
     -H "Content-Type: application/json" \
     -d '{
       "email": "your@northeastern.edu",
       "nickname": "your-nickname",
       "base_url": "http://'$ALB_URL'",
       "contract": "v1-album-store"
     }'
   ```

2. **Check results:**
   ```bash
   curl http://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/runs/{run_id}
   ```

## Cleanup

```bash
cd terraform
terraform destroy -var="environment=prod" -var="aws_region=us-west-2" -auto-approve
```

## Troubleshooting

### ECS Tasks Not Starting
- Check CloudWatch logs: `/ecs/prod-album-store`
- Verify ECR image exists: `aws ecr describe-images --repository-name prod-album-store`
- Check security groups allow ALB → ECS communication

### Photo Upload Fails
- Verify S3 bucket exists and has correct permissions
- Check IAM role has `s3:PutObject` and `s3:PutObjectAcl` permissions
- Review logs for S3 upload errors

### Sequence Numbers Not Incrementing
- Verify album exists before uploading photos
- Check DynamoDB table has `next_seq` attribute (initialized to 0)
- Review logs for `IncrementSeq` errors

### Health Check Failing
- Check ECS task is running: `aws ecs list-tasks --cluster prod-album-store-cluster`
- Verify port 8080 is exposed and Gin is listening
- Test directly: `curl http://{task-ip}:8080/health`

## Project Structure

```
FinalMastery/
├── src/
│   ├── main.go              # Server entry point, routing
│   ├── handlers.go          # HTTP handlers
│   ├── storage.go           # DynamoDB operations
│   ├── s3_client.go         # S3 upload/delete
│   ├── photo_processor.go   # Background photo processing
│   └── models.go            # Data structures
├── terraform/
│   ├── main.tf              # Root Terraform config
│   ├── variables.tf         # Input variables
│   ├── outputs.tf           # Outputs (ALB URL, etc.)
│   └── modules/
│       ├── dynamodb/        # DynamoDB tables
│       ├── s3/              # S3 bucket
│       ├── ecs/             # ECS cluster, service, task
│       ├── alb/             # Application Load Balancer
│       └── iam/             # IAM roles
├── docs/
│   ├── QUICKSTART.md        # Quick start guide
│   ├── IMPLEMENTATION.md    # Technical implementation details
│   ├── SUMMARY.md           # Project summary
│   └── 项目背景.md          # Chinese documentation
├── scripts/
│   ├── check_status.sh      # Check deployment status
│   └── update_resources.sh  # Update AWS resources
├── Dockerfile               # Multi-stage build
├── deploy.sh                # Automated deployment script
├── test_local.sh            # Local testing script
├── album-store.openapi.yaml # OpenAPI specification
├── album_store.md           # Assignment requirements
├── go.mod                   # Go dependencies
└── README.md                # This file
```

## Performance Characteristics

- **Photo Upload**: 202 response in < 100ms (async processing)
- **Photo Deletion**: < 2 seconds (parallel DynamoDB + S3)
- **Album List**: Paginated scan (handles 1000+ albums)
- **Sequence Increment**: Atomic DynamoDB operation (< 10ms)

## License

CS6650 Final Mastery Project - Northeastern University
