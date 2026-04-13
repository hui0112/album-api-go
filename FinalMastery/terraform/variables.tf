variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

variable "environment" {
  description = "Environment name (e.g., dev, prod)"
  type        = string
  default     = "prod"
}

variable "app_name" {
  description = "Application name"
  type        = string
  default     = "album-store"
}

variable "container_port" {
  description = "Port the container listens on"
  type        = number
  default     = 8080
}

variable "task_cpu" {
  description = "Fargate task CPU units (512=0.5vCPU, 1024=1vCPU, 2048=2vCPU)"
  type        = string
  default     = "2048"  # Increased from 512 to 2048 for better performance
}

variable "task_memory" {
  description = "Fargate task memory in MB"
  type        = string
  default     = "4096"  # Increased from 1024 to 4096 for large file handling
}

variable "desired_count" {
  description = "Desired number of ECS tasks"
  type        = number
  default     = 2
}
