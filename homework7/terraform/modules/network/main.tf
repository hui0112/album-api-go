# ============================================================================
# CUSTOM VPC NETWORK MODULE
# ============================================================================
#
# KEY DIFFERENCE FROM HW6:
# HW6 used the default VPC (already exists in every AWS account).
# HW7 creates a CUSTOM VPC with specific CIDR ranges as the instructions require.
#
# ARCHITECTURE:
# ┌─────────────────────────────────────────────────────────────┐
# │ VPC: 10.0.0.0/16 (65,536 IP addresses)                     │
# │                                                             │
# │ ┌─────────────────────┐  ┌─────────────────────┐           │
# │ │ Public Subnet 1     │  │ Public Subnet 2     │           │
# │ │ 10.0.1.0/24 (AZ-a) │  │ 10.0.2.0/24 (AZ-b) │           │
# │ │ - ALB               │  │ - ALB               │           │
# │ │ - NAT Gateway       │  │                     │           │
# │ └─────────┬───────────┘  └─────────────────────┘           │
# │           │                                                 │
# │ ┌─────────┴───────────┐  ┌─────────────────────┐           │
# │ │ Private Subnet 1    │  │ Private Subnet 2    │           │
# │ │ 10.0.10.0/24 (AZ-a) │  │ 10.0.11.0/24 (AZ-b)│           │
# │ │ - ECS Tasks         │  │ - ECS Tasks         │           │
# │ └─────────────────────┘  └─────────────────────┘           │
# └─────────────────────────────────────────────────────────────┘

# --------------------------------------------------------------------------
# DATA SOURCE: Get available AZs in the region
# --------------------------------------------------------------------------
# We need at least 2 AZs for ALB (high availability requirement).
data "aws_availability_zones" "available" {
  state = "available"
}

# --------------------------------------------------------------------------
# VPC
# --------------------------------------------------------------------------
# A VPC is your isolated network in AWS. All resources live inside it.
# CIDR 10.0.0.0/16 gives us 65,536 IP addresses (10.0.0.0 - 10.0.255.255).
#
# enable_dns_support + enable_dns_hostnames:
#   Allows resources in the VPC to resolve DNS names.
#   Required for ECS tasks to find AWS service endpoints (ECR, SQS, SNS).
resource "aws_vpc" "this" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${var.service_name}-vpc"
  }
}

# --------------------------------------------------------------------------
# INTERNET GATEWAY
# --------------------------------------------------------------------------
# An Internet Gateway connects your VPC to the public internet.
# Without it, nothing in the VPC can reach the internet (or be reached).
# Public subnets route through this. Private subnets do NOT.
resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.service_name}-igw"
  }
}

# --------------------------------------------------------------------------
# PUBLIC SUBNETS (for ALB + NAT Gateway)
# --------------------------------------------------------------------------
# Public subnets have a route to the Internet Gateway.
# ALB needs to be in public subnets to receive traffic from the internet.
# We create 2 in different AZs (ALB requires at least 2 AZs).
resource "aws_subnet" "public_1" {
  vpc_id                  = aws_vpc.this.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true # Resources here get public IPs automatically

  tags = {
    Name = "${var.service_name}-public-1"
  }
}

resource "aws_subnet" "public_2" {
  vpc_id                  = aws_vpc.this.id
  cidr_block              = "10.0.2.0/24"
  availability_zone       = data.aws_availability_zones.available.names[1]
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.service_name}-public-2"
  }
}

# --------------------------------------------------------------------------
# PRIVATE SUBNETS (for ECS tasks)
# --------------------------------------------------------------------------
# Private subnets have NO route to the Internet Gateway.
# ECS tasks run here — they can't be reached directly from the internet.
# They reach the internet OUTBOUND through the NAT Gateway.
resource "aws_subnet" "private_1" {
  vpc_id            = aws_vpc.this.id
  cidr_block        = "10.0.10.0/24"
  availability_zone = data.aws_availability_zones.available.names[0]

  tags = {
    Name = "${var.service_name}-private-1"
  }
}

resource "aws_subnet" "private_2" {
  vpc_id            = aws_vpc.this.id
  cidr_block        = "10.0.11.0/24"
  availability_zone = data.aws_availability_zones.available.names[1]

  tags = {
    Name = "${var.service_name}-private-2"
  }
}

# --------------------------------------------------------------------------
# NAT GATEWAY (allows private subnets to reach the internet)
# --------------------------------------------------------------------------
# WHY DO WE NEED THIS?
# ECS tasks in private subnets need outbound internet access to:
#   1. Pull Docker images from ECR
#   2. Call AWS APIs (SNS Publish, SQS ReceiveMessage)
#   3. Send CloudWatch logs
#
# The NAT Gateway sits in a PUBLIC subnet, has a public IP (Elastic IP),
# and forwards outbound traffic from private subnets to the internet.
# Inbound traffic from the internet is still BLOCKED.
#
# Cost: ~$0.045/hour (~$32/month). Remember to destroy when done!

# Elastic IP for the NAT Gateway (a static public IP address).
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.service_name}-nat-eip"
  }
}

# The NAT Gateway itself. Lives in public subnet 1.
resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public_1.id

  tags = {
    Name = "${var.service_name}-nat"
  }

  depends_on = [aws_internet_gateway.this]
}

# --------------------------------------------------------------------------
# ROUTE TABLES
# --------------------------------------------------------------------------
# Route tables define WHERE traffic goes.
# Think of them as "GPS directions" for network packets.

# PUBLIC route table: "To reach the internet, go through the Internet Gateway"
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"        # All internet-bound traffic...
    gateway_id = aws_internet_gateway.this.id  # ...goes through the IGW
  }

  tags = {
    Name = "${var.service_name}-public-rt"
  }
}

# PRIVATE route table: "To reach the internet, go through the NAT Gateway"
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block     = "0.0.0.0/0"         # All internet-bound traffic...
    nat_gateway_id = aws_nat_gateway.this.id  # ...goes through the NAT
  }

  tags = {
    Name = "${var.service_name}-private-rt"
  }
}

# Associate subnets with their route tables.
# This is what makes a subnet "public" or "private" — the route table it uses.
resource "aws_route_table_association" "public_1" {
  subnet_id      = aws_subnet.public_1.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "public_2" {
  subnet_id      = aws_subnet.public_2.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "private_1" {
  subnet_id      = aws_subnet.private_1.id
  route_table_id = aws_route_table.private.id
}

resource "aws_route_table_association" "private_2" {
  subnet_id      = aws_subnet.private_2.id
  route_table_id = aws_route_table.private.id
}

# --------------------------------------------------------------------------
# SECURITY GROUPS
# --------------------------------------------------------------------------

# ALB Security Group: allows port 80 from the internet
resource "aws_security_group" "alb" {
  name        = "${var.service_name}-alb-sg"
  description = "Allow HTTP traffic to ALB"
  vpc_id      = aws_vpc.this.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow HTTP from internet"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }

  tags = {
    Name = "${var.service_name}-alb-sg"
  }
}

# ECS Security Group: allows traffic from ALB only (more secure than HW6!)
# In HW6, port 8080 was open to 0.0.0.0/0. Here, only the ALB can reach ECS.
resource "aws_security_group" "ecs" {
  name        = "${var.service_name}-ecs-sg"
  description = "Allow traffic from ALB to ECS tasks"
  vpc_id      = aws_vpc.this.id

  ingress {
    from_port       = var.container_port
    to_port         = var.container_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]  # Only ALB can reach ECS!
    description     = "Allow traffic from ALB"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound (for AWS API calls, ECR pulls)"
  }

  tags = {
    Name = "${var.service_name}-ecs-sg"
  }
}
