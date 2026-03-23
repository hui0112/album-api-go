variable "service_name" {
  type        = string
  description = "Base name for log groups"
}

variable "retention_in_days" {
  type        = number
  default     = 7
  description = "How many days to keep logs"
}
