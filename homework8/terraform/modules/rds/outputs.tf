# The endpoint is the hostname used to connect to MySQL.
# Format: <identifier>.<random>.us-east-1.rds.amazonaws.com
output "endpoint" {
  description = "RDS endpoint (hostname for MySQL connection)"
  value       = aws_db_instance.mysql.address
}

output "port" {
  description = "RDS port (default MySQL: 3306)"
  value       = aws_db_instance.mysql.port
}

output "db_name" {
  description = "Database name"
  value       = aws_db_instance.mysql.db_name
}
