# ============================================================================
# ROOT OUTPUTS
# ============================================================================
# These values are printed after `terraform apply` completes.
# They give you the key information needed for testing.

output "alb_dns_name" {
  description = "ALB DNS — use this for curl and Locust tests"
  value       = module.alb.alb_dns_name
}

output "sns_topic_arn" {
  description = "SNS topic ARN (used by Order Receiver)"
  value       = module.messaging.sns_topic_arn
}

output "sqs_queue_url" {
  description = "SQS queue URL (used by Order Processor)"
  value       = module.messaging.sqs_queue_url
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = module.ecs_receiver.cluster_name
}

output "receiver_service_name" {
  description = "ECS service name for Order Receiver"
  value       = module.ecs_receiver.service_name
}

output "processor_service_name" {
  description = "ECS service name for Order Processor"
  value       = module.ecs_processor.service_name
}

output "lambda_function_name" {
  description = "Lambda function name (check CloudWatch logs at /aws/lambda/<this-name>)"
  value       = module.lambda.function_name
}
