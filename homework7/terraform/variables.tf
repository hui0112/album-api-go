# ============================================================================
# ROOT-LEVEL VARIABLES
# ============================================================================
# These are the "knobs" you can tweak without changing any module code.
# Each module receives the variables it needs from main.tf.

variable "aws_region" {
  type        = string
  default     = "us-east-1"
  description = "AWS region to deploy into"
}

variable "service_name" {
  type        = string
  default     = "hw7"
  description = "Base name for all resources. Used as a prefix."
}

variable "container_port" {
  type        = number
  default     = 8080
  description = "Port the Go apps listen on inside the container"
}

variable "log_retention_days" {
  type        = number
  default     = 7
  description = "How many days to keep CloudWatch logs"
}

# This controls the Order Processor's concurrency.
# Phase 3: 1 worker. Phase 5: try 5, 20, 100.
variable "worker_count" {
  type        = number
  default     = 1
  description = "Number of concurrent worker goroutines in the Order Processor"
}
