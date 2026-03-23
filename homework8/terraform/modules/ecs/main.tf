# ============================================================================
# ECS MODULE — Shopping Cart API Service
# ============================================================================
#
# KEY DESIGN: One service that can switch between MySQL and DynamoDB
# via the DB_TYPE environment variable. No code changes needed to swap.
#
# Environment variables injected:
#   DB_TYPE         = "mysql" or "dynamodb"
#   RDS_HOST        = MySQL endpoint (only used when DB_TYPE=mysql)
#   RDS_PORT        = "3306"
#   RDS_DATABASE    = "shopping_cart"
#   RDS_USER        = "admin"
#   RDS_PASSWORD    = (secret)
#   DYNAMODB_TABLE  = table name (only used when DB_TYPE=dynamodb)
#   AWS_REGION      = "us-east-1"

resource "aws_ecs_cluster" "this" {
  name = "${var.service_name}-cluster"
}

resource "aws_ecs_task_definition" "app" {
  family                   = "${var.service_name}-cart-api-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name      = "${var.service_name}-cart-api"
    image     = var.image
    essential = true

    portMappings = [{
      containerPort = var.container_port
    }]

    environment = [
      { name = "DB_TYPE", value = var.db_type },
      { name = "RDS_HOST", value = var.rds_host },
      { name = "RDS_PORT", value = var.rds_port },
      { name = "RDS_DATABASE", value = var.rds_database },
      { name = "RDS_USER", value = var.rds_user },
      { name = "RDS_PASSWORD", value = var.rds_password },
      { name = "DYNAMODB_TABLE", value = var.dynamodb_table },
      { name = "AWS_REGION", value = var.region },
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

resource "aws_ecs_service" "app" {
  name            = "${var.service_name}-cart-api"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.app.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = var.target_group_arn
    container_name   = "${var.service_name}-cart-api"
    container_port   = var.container_port
  }
}
