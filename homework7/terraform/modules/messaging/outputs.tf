output "sns_topic_arn" {
  description = "ARN of the SNS topic (Order Receiver publishes here)"
  value       = aws_sns_topic.orders.arn
}

output "sqs_queue_url" {
  description = "URL of the SQS queue (Order Processor polls this)"
  value       = aws_sqs_queue.orders.url
}

output "sqs_queue_arn" {
  description = "ARN of the SQS queue"
  value       = aws_sqs_queue.orders.arn
}
