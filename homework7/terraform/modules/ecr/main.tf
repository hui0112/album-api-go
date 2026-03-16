# ============================================================================
# ECR (Elastic Container Registry) MODULE
# ============================================================================
# ECR is AWS's Docker image registry — like Docker Hub but private.
# We need TWO repositories: one for Order Receiver, one for Order Processor.

resource "aws_ecr_repository" "receiver" {
  name = "${var.service_name}-receiver"
}

resource "aws_ecr_repository" "processor" {
  name = "${var.service_name}-processor"
}
