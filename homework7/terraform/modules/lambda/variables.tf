variable "service_name" {
  type        = string
  description = "Base name for resources"
}

variable "source_dir" {
  type        = string
  description = "Path to the Lambda Go source directory"
}

variable "execution_role_arn" {
  type        = string
  description = "IAM role ARN for Lambda execution"
}

variable "sns_topic_arn" {
  type        = string
  description = "ARN of the SNS topic to subscribe to"
}
