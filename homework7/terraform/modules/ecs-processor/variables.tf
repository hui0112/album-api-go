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

variable "cluster_id" {
  type        = string
  description = "ECS cluster ID (shared with Receiver)"
}

variable "sqs_queue_url" {
  type        = string
  description = "SQS queue URL to poll"
}

variable "worker_count" {
  type        = number
  default     = 1
  description = "Number of concurrent worker goroutines"
}

variable "cpu" {
  type    = string
  default = "256"
}

variable "memory" {
  type    = string
  default = "512"
}
