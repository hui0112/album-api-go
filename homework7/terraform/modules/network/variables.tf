variable "service_name" {
  type        = string
  description = "Base name for network resources"
}

variable "container_port" {
  type        = number
  default     = 8080
  description = "Port ECS containers listen on"
}
