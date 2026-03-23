# ============================================================================
# NETWORK MODULE (adapted from HW7)
# ============================================================================
#
# WHAT CHANGED FROM HW7:
# Added an RDS security group — allows ECS tasks to connect to MySQL on port 3306.
# Everything else (VPC, subnets, NAT, ALB SG, ECS SG) is identical to HW7.
#
# ARCHITECTURE:
# ┌─────────────────────────────────────────────────────────────┐
# │ VPC: 10.0.0.0/16                                           │
# │                                                             │
# │ ┌─────────────────────┐  ┌─────────────────────┐           │
# │ │ Public Subnet 1     │  │ Public Subnet 2     │           │
# │ │ 10.0.1.0/24 (AZ-a) │  │ 10.0.2.0/24 (AZ-b) │           │
# │ │ - ALB               │  │ - ALB               │           │
# │ │ - NAT Gateway       │  │                     │           │
# │ └─────────────────────┘  └─────────────────────┘           │
# │                                                             │
# │ ┌─────────────────────┐  ┌─────────────────────┐           │
# │ │ Private Subnet 1    │  │ Private Subnet 2    │           │
# │ │ 10.0.10.0/24 (AZ-a) │  │ 10.0.11.0/24 (AZ-b)│           │
# │ │ - ECS Tasks         │  │ - ECS Tasks         │           │
# │ │ - RDS (primary)     │  │ - RDS (standby)     │           │
# │ └─────────────────────┘  └─────────────────────┘           │
# └─────────────────────────────────────────────────────────────┘

data "aws_availability_zones" "available" {
  state = "available"
}

# --------------------------------------------------------------------------
# VPC
# --------------------------------------------------------------------------
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
resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.service_name}-igw"
  }
}

# --------------------------------------------------------------------------
# PUBLIC SUBNETS (for ALB + NAT Gateway)
# --------------------------------------------------------------------------
resource "aws_subnet" "public_1" {
  vpc_id                  = aws_vpc.this.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

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
# PRIVATE SUBNETS (for ECS tasks + RDS)
# --------------------------------------------------------------------------
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
# NAT GATEWAY
# --------------------------------------------------------------------------
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.service_name}-nat-eip"
  }
}

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
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = {
    Name = "${var.service_name}-public-rt"
  }
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this.id
  }

  tags = {
    Name = "${var.service_name}-private-rt"
  }
}

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

# ECS Security Group: allows traffic from ALB only
resource "aws_security_group" "ecs" {
  name        = "${var.service_name}-ecs-sg"
  description = "Allow traffic from ALB to ECS tasks"
  vpc_id      = aws_vpc.this.id

  ingress {
    from_port       = var.container_port
    to_port         = var.container_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
    description     = "Allow traffic from ALB"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound (for AWS API calls, ECR pulls, RDS)"
  }

  tags = {
    Name = "${var.service_name}-ecs-sg"
  }
}

# ★ NEW FOR HW8: RDS Security Group
# Only allows MySQL connections (port 3306) from ECS tasks.
# This is the "minimum privilege" principle — the database is only
# reachable by our application, not by the internet or other services.
resource "aws_security_group" "rds" {
  name        = "${var.service_name}-rds-sg"
  description = "Allow MySQL access from ECS tasks only"
  vpc_id      = aws_vpc.this.id

  ingress {
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = [aws_security_group.ecs.id]  # Only ECS can connect!
    description     = "Allow MySQL from ECS tasks"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }

  tags = {
    Name = "${var.service_name}-rds-sg"
  }
}
