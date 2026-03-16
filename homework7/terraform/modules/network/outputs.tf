output "vpc_id" {
  description = "ID of the custom VPC"
  value       = aws_vpc.this.id
}

# Public subnets — used by ALB
output "public_subnet_ids" {
  description = "IDs of public subnets (for ALB)"
  value       = [aws_subnet.public_1.id, aws_subnet.public_2.id]
}

# Private subnets — used by ECS tasks
output "private_subnet_ids" {
  description = "IDs of private subnets (for ECS)"
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
