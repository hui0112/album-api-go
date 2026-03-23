# ============================================================================
# ROOT MAIN.TF — WIRES ALL MODULES TOGETHER
# ============================================================================
#
# DEPENDENCY GRAPH:
#
#   network ──► alb ──► ecs
#      │                  │
#      │                  ├── ecr (Docker image)
#      │                  ├── logging (CloudWatch logs)
#      │                  ├── rds (MySQL endpoint + credentials)
#      │                  └── dynamodb (table name)
#      │
#      └──► rds (needs private subnets + RDS security group)
#
# HOW TO SWITCH DATABASES:
#   terraform apply -var="db_type=mysql"    -var="db_password=YourPass123!"
#   terraform apply -var="db_type=dynamodb" -var="db_password=YourPass123!"
#
# Both RDS and DynamoDB are ALWAYS created (for comparison).
# The db_type variable only controls which one the Go app connects to.

# --------------------------------------------------------------------------
# TERRAFORM + PROVIDER CONFIGURATION
# --------------------------------------------------------------------------
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 2.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# Docker provider — authenticates with ECR to push images
data "aws_ecr_authorization_token" "registry" {}

provider "docker" {
  registry_auth {
    address  = data.aws_ecr_authorization_token.registry.proxy_endpoint
    username = data.aws_ecr_authorization_token.registry.user_name
    password = data.aws_ecr_authorization_token.registry.password
  }
}

# --------------------------------------------------------------------------
# IAM ROLE — LabRole (pre-existing in AWS Academy)
# --------------------------------------------------------------------------
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# --------------------------------------------------------------------------
# 1. NETWORK — VPC, subnets, security groups (including RDS SG)
# --------------------------------------------------------------------------
module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

# --------------------------------------------------------------------------
# 2. ECR — Docker image repository
# --------------------------------------------------------------------------
module "ecr" {
  source       = "./modules/ecr"
  service_name = var.service_name
}

# --------------------------------------------------------------------------
# 3. LOGGING — CloudWatch log group
# --------------------------------------------------------------------------
module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# --------------------------------------------------------------------------
# 4. ALB — Application Load Balancer (public entry point)
# --------------------------------------------------------------------------
module "alb" {
  source                = "./modules/alb"
  service_name          = var.service_name
  vpc_id                = module.network.vpc_id
  public_subnet_ids     = module.network.public_subnet_ids
  alb_security_group_id = module.network.alb_security_group_id
  container_port        = var.container_port
}

# --------------------------------------------------------------------------
# 5. RDS — MySQL Database (always created for comparison)
# --------------------------------------------------------------------------
module "rds" {
  source                = "./modules/rds"
  service_name          = var.service_name
  private_subnet_ids    = module.network.private_subnet_ids
  rds_security_group_id = module.network.rds_security_group_id
  db_name               = "shopping_cart"
  db_username           = "admin"
  db_password           = var.db_password
}

# --------------------------------------------------------------------------
# 6. DYNAMODB — NoSQL Table (always created for comparison)
# --------------------------------------------------------------------------
module "dynamodb" {
  source       = "./modules/dynamodb"
  service_name = var.service_name
}

# --------------------------------------------------------------------------
# 7. ECS — Shopping Cart API Service
# --------------------------------------------------------------------------
module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  private_subnet_ids = module.network.private_subnet_ids
  security_group_ids = [module.network.ecs_security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.log_group_name
  region             = var.aws_region
  target_group_arn   = module.alb.target_group_arn

  # Database configuration — which backend to use
  db_type        = var.db_type
  rds_host       = module.rds.endpoint
  rds_port       = tostring(module.rds.port)
  rds_database   = module.rds.db_name
  rds_user       = "admin"
  rds_password   = var.db_password
  dynamodb_table = module.dynamodb.table_name
}

# --------------------------------------------------------------------------
# 8. BUILD & PUSH DOCKER IMAGE
# --------------------------------------------------------------------------
resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"
  build {
    context = "../src"
  }
}

resource "docker_registry_image" "app" {
  name = docker_image.app.name
}
