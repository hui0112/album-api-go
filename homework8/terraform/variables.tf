# ============================================================================
# ROOT-LEVEL VARIABLES
# ============================================================================

variable "aws_region" {
  type        = string
  default     = "us-east-1"
  description = "AWS region to deploy into"
}

variable "service_name" {
  type        = string
  default     = "hw8"
  description = "Base name for all resources"
}

variable "container_port" {
  type        = number
  default     = 8080
  description = "Port the Go app listens on"
}

variable "log_retention_days" {
  type        = number
  default     = 7
  description = "How many days to keep CloudWatch logs"
}

# ★ NEW FOR HW8: Database selection
variable "db_type" {
  type        = string
  default     = "mysql"
  description = "Which database backend to use: 'mysql' or 'dynamodb'"

  validation {
    condition     = contains(["mysql", "dynamodb"], var.db_type)
    error_message = "db_type must be 'mysql' or 'dynamodb'"
  }
}

variable "db_password" {
  type        = string
  sensitive   = true
  description = "Password for the RDS MySQL database"
}
