variable "service_name" {
  type        = string
  description = "Base name for ALB resources"
}

variable "vpc_id" {
  type        = string
  description = "VPC ID for the ALB security group and target group"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for the ALB"
}

variable "container_port" {
  type        = number
  description = "Port the ECS containers listen on"
}
