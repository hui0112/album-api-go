# ============================================================================
# RDS MODULE — MySQL Database (★ NEW FOR HW8)
# ============================================================================
#
# This creates a MySQL 8.0 database on the smallest instance (db.t3.micro).
#
# WHY RDS INSTEAD OF SELF-MANAGED MySQL?
# RDS handles backups, patching, failover automatically.
# We just define what we want; AWS manages everything else.
#
# NETWORK PLACEMENT:
# The RDS instance sits in PRIVATE subnets — it cannot be reached from
# the internet. Only ECS tasks (via the RDS security group) can connect.

# DB Subnet Group — tells RDS which subnets it can use.
# AWS requires at least 2 subnets in different Availability Zones
# for high availability (even if we're using a single-AZ instance).
resource "aws_db_subnet_group" "this" {
  name       = "${var.service_name}-db-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name = "${var.service_name}-db-subnet-group"
  }
}

# The MySQL RDS instance.
resource "aws_db_instance" "mysql" {
  identifier     = "${var.service_name}-mysql"
  engine         = "mysql"
  engine_version = "8.0"
  instance_class = "db.t3.micro"   # Free Tier eligible: 1 vCPU, 1 GB RAM

  allocated_storage = 20            # 20 GB storage (Free Tier: up to 20 GB)
  storage_type      = "gp2"         # General Purpose SSD

  db_name  = var.db_name             # Creates this database on startup
  username = var.db_username
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [var.rds_security_group_id]

  # Assignment settings — NOT for production!
  skip_final_snapshot    = true   # Don't create snapshot when deleting
  deletion_protection    = false  # Allow terraform destroy to delete it
  publicly_accessible    = false  # No public access — private subnet only

  tags = {
    Name = "${var.service_name}-mysql"
  }
}
