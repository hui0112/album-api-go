resource "aws_dynamodb_table" "albums" {
  name         = "${var.environment}-albums"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  tags = {
    Name        = "${var.environment}-albums"
    Environment = var.environment
  }
}

resource "aws_dynamodb_table" "photos" {
  name         = "${var.environment}-photos"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "photo_id"

  attribute {
    name = "photo_id"
    type = "S"
  }

  attribute {
    name = "album_id"
    type = "S"
  }

  global_secondary_index {
    name            = "album_id-index"
    hash_key        = "album_id"
    projection_type = "ALL"
  }

  tags = {
    Name        = "${var.environment}-photos"
    Environment = var.environment
  }
}
