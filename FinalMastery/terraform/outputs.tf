output "alb_url" {
  description = "URL of the Application Load Balancer"
  value       = module.alb.alb_dns_name
}

output "albums_table_name" {
  description = "Name of the albums DynamoDB table"
  value       = module.dynamodb.albums_table_name
}

output "photos_table_name" {
  description = "Name of the photos DynamoDB table"
  value       = module.dynamodb.photos_table_name
}

output "s3_bucket_name" {
  description = "Name of the S3 bucket for photos"
  value       = module.s3.bucket_name
}

output "ecr_repository_url" {
  description = "URL of the ECR repository"
  value       = aws_ecr_repository.app.repository_url
}
