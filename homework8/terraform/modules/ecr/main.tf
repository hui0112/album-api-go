# ============================================================================
# ECR MODULE — Docker Image Repository
# ============================================================================
# Only ONE repository this time (HW7 had two: receiver + processor).
# We have a single shopping cart API service.

resource "aws_ecr_repository" "app" {
  name = "${var.service_name}-cart-api"
}
