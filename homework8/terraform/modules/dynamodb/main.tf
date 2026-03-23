# ============================================================================
# DYNAMODB MODULE (★ NEW FOR HW8)
# ============================================================================
#
# CONTRAST WITH RDS:
# - No subnets, no security groups needed (DynamoDB is a managed AWS endpoint)
# - Access controlled by IAM roles, not network rules
# - No instance to manage — fully serverless
# - Creates in seconds (vs 5-10 min for RDS)
#
# TABLE DESIGN:
# Partition Key = cart_id (String)
# No Sort Key — each cart is one Item with embedded items list.
# Billing = PAY_PER_REQUEST (auto-scales, no capacity planning needed).

resource "aws_dynamodb_table" "shopping_carts" {
  name         = "${var.service_name}-shopping-carts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "cart_id"

  attribute {
    name = "cart_id"
    type = "S"   # S = String
  }

  # NOTE: Only key/index attributes are declared here.
  # Other attributes (customer_id, items, created_at) are schema-free —
  # you just include them when writing Items, no need to pre-define.

  tags = {
    Name = "${var.service_name}-shopping-carts"
  }
}
