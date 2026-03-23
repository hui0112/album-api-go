variable "service_name" {
  type = string
}

variable "image" {
  type        = string
  description = "ECR image URI with tag"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "Private subnets for ECS tasks"
}

variable "security_group_ids" {
  type        = list(string)
  description = "Security groups for ECS tasks"
}

variable "execution_role_arn" {
  type = string
}

variable "task_role_arn" {
  type = string
}

variable "log_group_name" {
  type = string
}

variable "region" {
  type = string
}

variable "target_group_arn" {
  type        = string
  description = "ALB target group ARN"
}

# Database configuration
variable "db_type" {
  type        = string
  description = "Database type: mysql or dynamodb"
}

variable "rds_host" {
  type        = string
  default     = ""
  description = "RDS endpoint hostname"
}

variable "rds_port" {
  type        = string
  default     = "3306"
  description = "RDS port"
}

variable "rds_database" {
  type        = string
  default     = "shopping_cart"
  description = "RDS database name"
}

variable "rds_user" {
  type        = string
  default     = "admin"
  description = "RDS username"
}

variable "rds_password" {
  type        = string
  default     = ""
  sensitive   = true
  description = "RDS password"
}

variable "dynamodb_table" {
  type        = string
  default     = ""
  description = "DynamoDB table name"
}

variable "cpu" {
  type    = string
  default = "256"
}

variable "memory" {
  type    = string
  default = "512"
}
