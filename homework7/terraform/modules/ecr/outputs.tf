output "receiver_repository_url" {
  description = "ECR repository URL for Order Receiver"
  value       = aws_ecr_repository.receiver.repository_url
}

output "processor_repository_url" {
  description = "ECR repository URL for Order Processor"
  value       = aws_ecr_repository.processor.repository_url
}
