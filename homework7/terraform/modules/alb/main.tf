# ============================================================================
# ALB (Application Load Balancer) MODULE
# ============================================================================
# The ALB is the public entry point for HTTP traffic.
#
# Traffic flow:
#   Internet → ALB (port 80) → Target Group → ECS Tasks (port 8080)
#
# KEY DIFFERENCE FROM HW6:
# - HW6 created the ALB security group inside this module
# - HW7 receives the ALB security group from the network module
#   (because the ECS security group references the ALB SG, so they
#   need to be created together in the network module)

# Application Load Balancer
resource "aws_lb" "this" {
  name               = "${var.service_name}-alb"
  internal           = false          # Public-facing (internet accessible)
  load_balancer_type = "application"
  security_groups    = [var.alb_security_group_id]
  subnets            = var.public_subnet_ids  # ALB needs public subnets

  tags = {
    Name = "${var.service_name}-alb"
  }
}

# Target Group — tells ALB where to send traffic
# target_type = "ip" is required for Fargate (containers get dynamic IPs)
resource "aws_lb_target_group" "this" {
  name        = "${var.service_name}-tg"
  port        = var.container_port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "ip"

  # Health check: ALB pings /health every 30 seconds.
  # If 2 consecutive checks return 200, the task is "healthy."
  # If 3 consecutive checks fail, the task is "unhealthy" and removed.
  health_check {
    path                = "/health"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
    matcher             = "200"
  }
}

# Listener — the ALB listens on port 80 and forwards to the target group
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }
}
