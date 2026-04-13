# Use existing LabRole instead of creating new roles
  # This is compatible with AWS Academy/voclabs accounts

  data "aws_iam_role" "lab_role" {
    name = "LabRole"
  }

  # No resources to create - just use the existing LabRole