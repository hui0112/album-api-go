# ============================================================================
# ROOT MAIN.TF — ORCHESTRATES ALL MODULES
# ============================================================================
#
# This file wires all modules together. Think of it as the "wiring diagram"
# that connects networking, load balancing, containers, and messaging.
#
# DEPENDENCY GRAPH (what depends on what):
#
#   network ──► alb ──► ecs-receiver
#      │                     │
#      │                     ├── ecr (receiver image)
#      │                     ├── logging (receiver logs)
#      │                     └── messaging (SNS topic ARN)
#      │
#      └──────► ecs-processor
#                    │
#                    ├── ecr (processor image)
#                    ├── logging (processor logs)
#                    ├── messaging (SQS queue URL)
#                    └── ecs-receiver (cluster ID — shared cluster)

# --------------------------------------------------------------------------
# 1. NETWORK — VPC, subnets, security groups
# --------------------------------------------------------------------------
module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

# --------------------------------------------------------------------------
# 2. ECR — Docker image repositories (one per service)
# --------------------------------------------------------------------------
module "ecr" {
  source       = "./modules/ecr"
  service_name = var.service_name
}

# --------------------------------------------------------------------------
# 3. LOGGING — CloudWatch log groups (one per service)
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
# 5. MESSAGING — SNS topic + SQS queue
# --------------------------------------------------------------------------
module "messaging" {
  source       = "./modules/messaging"
  service_name = var.service_name
}

# --------------------------------------------------------------------------
# IAM ROLE — LabRole (pre-existing in AWS Academy)
# --------------------------------------------------------------------------
# data source looks up an existing IAM role by name (doesn't create it).
# LabRole is pre-configured in AWS Academy with permissions for ECS, ECR,
# SNS, SQS, CloudWatch, etc.
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# --------------------------------------------------------------------------
# 6. ECS - ORDER RECEIVER (handles HTTP traffic via ALB)
# --------------------------------------------------------------------------
module "ecs_receiver" {
  source             = "./modules/ecs-receiver"
  service_name       = var.service_name
  image              = "${module.ecr.receiver_repository_url}:latest"
  container_port     = var.container_port
  private_subnet_ids = module.network.private_subnet_ids
  security_group_ids = [module.network.ecs_security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.receiver_log_group_name
  region             = var.aws_region
  target_group_arn   = module.alb.target_group_arn
  sns_topic_arn      = module.messaging.sns_topic_arn
}

# --------------------------------------------------------------------------
# 7. ECS - ORDER PROCESSOR (polls SQS, no ALB)
# --------------------------------------------------------------------------
module "ecs_processor" {
  source             = "./modules/ecs-processor"
  service_name       = var.service_name
  image              = "${module.ecr.processor_repository_url}:latest"
  container_port     = var.container_port
  private_subnet_ids = module.network.private_subnet_ids
  security_group_ids = [module.network.ecs_security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.processor_log_group_name
  region             = var.aws_region
  cluster_id         = module.ecs_receiver.cluster_id
  sqs_queue_url      = module.messaging.sqs_queue_url
  worker_count       = var.worker_count
}

# --------------------------------------------------------------------------
# 8. BUILD & PUSH DOCKER IMAGES
# --------------------------------------------------------------------------
# Terraform's Docker provider builds images locally and pushes to ECR.
# This happens as part of `terraform apply`.

# Build Order Receiver image
resource "docker_image" "receiver" {
  name = "${module.ecr.receiver_repository_url}:latest"
  build {
    context = "../order-receiver"
  }
}

# Push Order Receiver image to ECR
resource "docker_registry_image" "receiver" {
  name = docker_image.receiver.name
}

# Build Order Processor image
resource "docker_image" "processor" {
  name = "${module.ecr.processor_repository_url}:latest"
  build {
    context = "../order-processor"
  }
}

# Push Order Processor image to ECR
resource "docker_registry_image" "processor" {
  name = docker_image.processor.name
}

# --------------------------------------------------------------------------
# 9. LAMBDA — Serverless Order Processor (Part III)
# --------------------------------------------------------------------------
# This adds a Lambda function that ALSO subscribes to the same SNS topic.
# Both the ECS processor and Lambda will receive orders simultaneously.
# In a real migration, you'd remove the ECS processor after validation.
module "lambda" {
  source             = "./modules/lambda"
  service_name       = var.service_name
  source_dir         = "${path.module}/../order-lambda"
  execution_role_arn = data.aws_iam_role.lab_role.arn
  sns_topic_arn      = module.messaging.sns_topic_arn
}
