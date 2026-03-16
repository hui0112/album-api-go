# ============================================================================
# PROVIDER CONFIGURATION
# ============================================================================
# Terraform needs to know which "providers" (plugins) to use.
# We need two:
#   1. AWS provider: creates AWS resources (VPC, ECS, SNS, SQS, etc.)
#   2. Docker provider: builds and pushes Docker images to ECR
#
# The Docker provider authenticates against ECR using a temporary token
# so Terraform can push images as part of `terraform apply`.

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.7.0"
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

# Fetch a temporary ECR auth token for Docker to push images.
data "aws_ecr_authorization_token" "registry" {}

# Configure Docker provider to authenticate against our ECR registry.
provider "docker" {
  registry_auth {
    address  = data.aws_ecr_authorization_token.registry.proxy_endpoint
    username = data.aws_ecr_authorization_token.registry.user_name
    password = data.aws_ecr_authorization_token.registry.password
  }
}
