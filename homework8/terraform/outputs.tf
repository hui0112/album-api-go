# ============================================================================
# ROOT OUTPUTS — printed after terraform apply
# ============================================================================

output "alb_dns_name" {
  description = "ALB URL — use this for API calls and testing"
  value       = "http://${module.alb.alb_dns_name}"
}

output "db_type" {
  description = "Currently active database backend"
  value       = var.db_type
}

output "rds_endpoint" {
  description = "RDS MySQL endpoint"
  value       = module.rds.endpoint
}

output "dynamodb_table" {
  description = "DynamoDB table name"
  value       = module.dynamodb.table_name
}

output "ecs_cluster" {
  description = "ECS cluster name"
  value       = module.ecs.cluster_name
}
