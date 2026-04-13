variable "environment" {
  description = "Environment name"
  type        = string
}

variable "app_name" {
  description = "Application name"
  type        = string
}

variable "albums_table_arn" {
  description = "ARN of the albums DynamoDB table"
  type        = string
}

variable "photos_table_arn" {
  description = "ARN of the photos DynamoDB table"
  type        = string
}

variable "s3_bucket_arn" {
  description = "ARN of the S3 bucket"
  type        = string
}
