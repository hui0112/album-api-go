output "receiver_log_group_name" {
  description = "CloudWatch log group for Order Receiver"
  value       = aws_cloudwatch_log_group.receiver.name
}

output "processor_log_group_name" {
  description = "CloudWatch log group for Order Processor"
  value       = aws_cloudwatch_log_group.processor.name
}
