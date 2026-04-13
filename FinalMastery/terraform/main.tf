terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# Random suffix for unique resource names
resource "random_string" "suffix" {
  length  = 8
  special = false
  upper   = false
}

# Get default VPC
data "aws_vpc" "default" {
  default = true
}

# Get default subnets
data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# ECR Repository
resource "aws_ecr_repository" "app" {
  name                 = "${var.environment}-${var.app_name}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = {
    Name        = "${var.environment}-${var.app_name}"
    Environment = var.environment
  }
}

# DynamoDB Tables
module "dynamodb" {
  source = "./modules/dynamodb"

  environment = var.environment
}

# S3 Bucket
module "s3" {
  source = "./modules/s3"

  environment = var.environment
  suffix      = random_string.suffix.result
}

# IAM Roles
module "iam" {
  source = "./modules/iam"

  environment       = var.environment
  app_name          = var.app_name
  albums_table_arn  = module.dynamodb.albums_table_arn
  photos_table_arn  = module.dynamodb.photos_table_arn
  s3_bucket_arn     = module.s3.bucket_arn
}

# Application Load Balancer
module "alb" {
  source = "./modules/alb"

  environment       = var.environment
  app_name          = var.app_name
  vpc_id            = data.aws_vpc.default.id
  public_subnet_ids = data.aws_subnets.default.ids
  container_port    = var.container_port
}

# ECS Cluster and Service
module "ecs" {
  source = "./modules/ecs"

  environment            = var.environment
  app_name               = var.app_name
  vpc_id                 = data.aws_vpc.default.id
  private_subnet_ids     = data.aws_subnets.default.ids
  alb_security_group_id  = module.alb.alb_security_group_id
  container_image        = "${aws_ecr_repository.app.repository_url}:latest"
  container_port         = var.container_port
  task_cpu               = var.task_cpu
  task_memory            = var.task_memory
  desired_count          = var.desired_count
  execution_role_arn     = module.iam.ecs_task_execution_role_arn
  task_role_arn          = module.iam.ecs_task_role_arn
  albums_table_name      = module.dynamodb.albums_table_name
  photos_table_name      = module.dynamodb.photos_table_name
  s3_bucket_name         = module.s3.bucket_name
  aws_region             = var.aws_region
  target_group_arn       = module.alb.target_group_arn
  alb_listener_arn       = module.alb.alb_arn
}
