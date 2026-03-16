# ============================================================================
# ECS MODULE - ORDER RECEIVER
# ============================================================================
# This creates the ECS service that runs the Order Receiver.
# It's connected to the ALB and handles incoming HTTP traffic.
#
# KEY DIFFERENCE FROM HW6:
# - Passes SNS_TOPIC_ARN as an environment variable to the container
# - No auto-scaling (homework asks for 1 task)
# - Uses private subnets (no public IP, traffic comes through ALB)

# ECS Cluster — a logical grouping of tasks/services.
# Both Receiver and Processor share this cluster.
resource "aws_ecs_cluster" "this" {
  name = "${var.service_name}-cluster"
}

# Task Definition — the "blueprint" for running a container.
# Defines: what image, how much CPU/memory, environment variables, logging.
resource "aws_ecs_task_definition" "receiver" {
  family                   = "${var.service_name}-receiver-task"
  network_mode             = "awsvpc"        # Required for Fargate
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu          # 256 = 0.25 vCPU
  memory                   = var.memory       # 512 MiB

  # execution_role: allows ECS to pull images from ECR and write logs
  # task_role: allows the running container to call AWS services (SNS)
  execution_role_arn = var.execution_role_arn
  task_role_arn      = var.task_role_arn

  container_definitions = jsonencode([{
    name      = "${var.service_name}-receiver"
    image     = var.image
    essential = true    # If this container dies, the task stops

    portMappings = [{
      containerPort = var.container_port
    }]

    # Environment variables injected into the container.
    # The Go app reads SNS_TOPIC_ARN with os.Getenv("SNS_TOPIC_ARN").
    environment = [
      {
        name  = "SNS_TOPIC_ARN"
        value = var.sns_topic_arn
      },
      {
        name  = "AWS_REGION"
        value = var.region
      }
    ]

    # CloudWatch logging — container stdout/stderr goes to CloudWatch.
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

# ECS Service — keeps the desired number of tasks running.
# If a task crashes, ECS automatically starts a new one.
resource "aws_ecs_service" "receiver" {
  name            = "${var.service_name}-receiver"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.receiver.arn
  desired_count   = 1               # 1 task as per homework spec
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = false  # Private subnet, no public IP needed
  }

  # Connect to ALB — tells ECS to register tasks with the target group.
  # ALB routes incoming HTTP traffic to this container on port 8080.
  load_balancer {
    target_group_arn = var.target_group_arn
    container_name   = "${var.service_name}-receiver"
    container_port   = var.container_port
  }
}
