output "ecs_task_execution_role_arn" {
    description = "ARN of the ECS task execution role (LabRole)"
    value       = data.aws_iam_role.lab_role.arn
  }

  output "ecs_task_role_arn" {
    description = "ARN of the ECS task role (LabRole)"
    value       = data.aws_iam_role.lab_role.arn
  }