output "albums_table_name" {
  description = "Name of the albums table"
  value       = aws_dynamodb_table.albums.name
}

output "albums_table_arn" {
  description = "ARN of the albums table"
  value       = aws_dynamodb_table.albums.arn
}

output "photos_table_name" {
  description = "Name of the photos table"
  value       = aws_dynamodb_table.photos.name
}

output "photos_table_arn" {
  description = "ARN of the photos table"
  value       = aws_dynamodb_table.photos.arn
}
