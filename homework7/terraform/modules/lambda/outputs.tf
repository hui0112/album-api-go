output "function_name" {
  description = "Lambda function name (use for CloudWatch log lookup)"
  value       = aws_lambda_function.order_processor.function_name
}

output "function_arn" {
  description = "Lambda function ARN"
  value       = aws_lambda_function.order_processor.arn
}
