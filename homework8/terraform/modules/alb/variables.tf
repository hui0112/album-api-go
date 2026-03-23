variable "service_name" {
  type        = string
  description = "Base name for ALB resources"
}

variable "vpc_id" {
  type        = string
  description = "VPC ID for target group"
}

variable "public_subnet_ids" {
  type        = list(string)
  description = "Public subnet IDs for ALB placement"
}

variable "alb_security_group_id" {
  type        = string
  description = "Security group ID for ALB"
}

variable "container_port" {
  type        = number
  default     = 8080
  description = "Port containers listen on"
}
