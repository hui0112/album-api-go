#!/bin/bash
set -e

# Album Store Deployment Script

ENVIRONMENT="${1:-prod}"
AWS_REGION="${2:-us-west-2}"
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

echo "========================================="
echo "Album Store Deployment"
echo "========================================="
echo "Environment: $ENVIRONMENT"
echo "Region: $AWS_REGION"
echo "AWS Account: $AWS_ACCOUNT_ID"
echo "========================================="

# Step 1: Initialize Terraform
echo ""
echo "Step 1: Initializing Terraform..."
cd terraform
terraform init

# Step 2: Apply Terraform
echo ""
echo "Step 2: Creating AWS infrastructure..."
terraform apply -var="environment=$ENVIRONMENT" -var="aws_region=$AWS_REGION" -auto-approve

# Get outputs
ECR_REPO=$(terraform output -raw ecr_repository_url)
ALB_URL=$(terraform output -raw alb_url)

echo ""
echo "ECR Repository: $ECR_REPO"
echo "ALB URL: http://$ALB_URL"

cd ..

# Step 3: Build Docker image
echo ""
echo "Step 3: Building Docker image..."
docker build -t album-store:latest .

# Step 4: Tag and push to ECR
echo ""
echo "Step 4: Pushing image to ECR..."
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $ECR_REPO
docker tag album-store:latest $ECR_REPO:latest
docker push $ECR_REPO:latest

# Step 5: Force ECS service update
echo ""
echo "Step 5: Updating ECS service..."
CLUSTER_NAME="${ENVIRONMENT}-album-store-cluster"
SERVICE_NAME="${ENVIRONMENT}-album-store-service"

aws ecs update-service \
    --cluster $CLUSTER_NAME \
    --service $SERVICE_NAME \
    --force-new-deployment \
    --region $AWS_REGION

echo ""
echo "========================================="
echo "Deployment Complete!"
echo "========================================="
echo "ALB URL: http://$ALB_URL"
echo ""
echo "Wait 2-3 minutes for the service to be healthy, then test:"
echo "  curl http://$ALB_URL/health"
echo ""
echo "To view logs:"
echo "  aws logs tail /ecs/$ENVIRONMENT-album-store --follow --region $AWS_REGION"
echo "========================================="
