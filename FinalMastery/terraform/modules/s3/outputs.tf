output "bucket_name" {
  description = "Name of the S3 bucket"
  value       = aws_s3_bucket.photos.bucket
}

output "bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.photos.arn
}

output "bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  value       = aws_s3_bucket.photos.bucket_domain_name
}
