# ============================================================================
# CLOUDWATCH LOGGING MODULE
# ============================================================================
# CloudWatch Logs collects stdout/stderr from ECS containers.
# We create separate log groups for each service for easier debugging.

resource "aws_cloudwatch_log_group" "receiver" {
  name              = "/ecs/${var.service_name}-receiver"
  retention_in_days = var.retention_in_days
}

resource "aws_cloudwatch_log_group" "processor" {
  name              = "/ecs/${var.service_name}-processor"
  retention_in_days = var.retention_in_days
}
