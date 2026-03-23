output "repository_url" {
  description = "ECR repository URL for the shopping cart API"
  value       = aws_ecr_repository.app.repository_url
}
