# ============================================================================
# CLOUDWATCH LOGGING MODULE
# ============================================================================
# One log group for the shopping cart API service.

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.service_name}-cart-api"
  retention_in_days = var.retention_in_days
}
