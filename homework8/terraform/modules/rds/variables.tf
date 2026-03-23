variable "service_name" {
  type        = string
  description = "Base name for RDS resources"
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "Private subnet IDs for DB subnet group"
}

variable "rds_security_group_id" {
  type        = string
  description = "Security group allowing MySQL access from ECS"
}

variable "db_name" {
  type        = string
  default     = "shopping_cart"
  description = "Name of the database to create"
}

variable "db_username" {
  type        = string
  default     = "admin"
  description = "Master username for the database"
}

variable "db_password" {
  type        = string
  sensitive   = true
  description = "Master password for the database (do NOT hardcode!)"
}
