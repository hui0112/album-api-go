output "vpc_id" {
  description = "ID of the custom VPC"
  value       = aws_vpc.this.id
}

output "public_subnet_ids" {
  description = "IDs of public subnets (for ALB)"
  value       = [aws_subnet.public_1.id, aws_subnet.public_2.id]
}

output "private_subnet_ids" {
  description = "IDs of private subnets (for ECS + RDS)"
  value       = [aws_subnet.private_1.id, aws_subnet.private_2.id]
}

output "alb_security_group_id" {
  description = "Security group for ALB"
  value       = aws_security_group.alb.id
}

output "ecs_security_group_id" {
  description = "Security group for ECS tasks"
  value       = aws_security_group.ecs.id
}

# ★ NEW FOR HW8
output "rds_security_group_id" {
  description = "Security group for RDS (allows MySQL from ECS only)"
  value       = aws_security_group.rds.id
}
