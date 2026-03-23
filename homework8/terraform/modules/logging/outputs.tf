output "log_group_name" {
  description = "CloudWatch log group for shopping cart API"
  value       = aws_cloudwatch_log_group.app.name
}
