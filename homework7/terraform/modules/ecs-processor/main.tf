# ============================================================================
# ECS MODULE - ORDER PROCESSOR
# ============================================================================
# This creates the ECS service that runs the Order Processor (SQS consumer).
#
# KEY DIFFERENCES FROM THE RECEIVER:
# 1. NO load_balancer block — this service doesn't receive HTTP traffic
#    from the ALB. It only polls SQS internally.
# 2. Passes SQS_QUEUE_URL and WORKER_COUNT as environment variables.
# 3. Still has a /health endpoint for ECS health monitoring, but ECS
#    checks it directly (not through ALB).
# 4. Shares the same cluster as the Receiver.

# Task Definition for the Order Processor
resource "aws_ecs_task_definition" "processor" {
  family                   = "${var.service_name}-processor-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory

  execution_role_arn = var.execution_role_arn
  task_role_arn      = var.task_role_arn

  container_definitions = jsonencode([{
    name      = "${var.service_name}-processor"
    image     = var.image
    essential = true

    portMappings = [{
      containerPort = var.container_port
    }]

    # These environment variables configure the processor:
    # - SQS_QUEUE_URL: where to poll for messages
    # - WORKER_COUNT: how many concurrent goroutines to spawn
    # - AWS_REGION: needed for AWS SDK
    environment = [
      {
        name  = "SQS_QUEUE_URL"
        value = var.sqs_queue_url
      },
      {
        name  = "WORKER_COUNT"
        value = tostring(var.worker_count)
      },
      {
        name  = "AWS_REGION"
        value = var.region
      }
    ]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.log_group_name
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

# ECS Service for the Processor.
# Note: NO load_balancer block! This service doesn't serve HTTP traffic.
# It only polls SQS and processes messages internally.
resource "aws_ecs_service" "processor" {
  name            = "${var.service_name}-processor"
  cluster         = var.cluster_id    # Share cluster with Receiver
  task_definition = aws_ecs_task_definition.processor.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = false
  }
}
