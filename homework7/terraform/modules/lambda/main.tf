# ============================================================================
# LAMBDA MODULE — Serverless Order Processor
# ============================================================================
#
# This replaces the ECS Order Processor (Part II) with a Lambda function.
#
# PART II (ECS):   SNS → SQS → ECS Processor (you manage polling, scaling, health)
# PART III (Lambda): SNS → Lambda (AWS manages everything)
#
# HOW IT WORKS:
# 1. Terraform builds the Go binary (cross-compiled for Linux/amd64)
# 2. Packages it as a ZIP file (Lambda deployment format)
# 3. Creates the Lambda function with the ZIP
# 4. Subscribes the Lambda to the SNS topic
# 5. When an order arrives on SNS, AWS automatically invokes the Lambda
#
# WHY "provided.al2023"?
# Lambda needs a runtime to execute your code. For Go, we use "provided.al2023"
# which means: "I'm providing my own binary (not Python/Node/Java)."
# The binary MUST be named "bootstrap" — that's what Lambda looks for.

# --------------------------------------------------------------------------
# BUILD THE GO BINARY
# --------------------------------------------------------------------------
# Cross-compile for Linux (Lambda runs on Amazon Linux, not macOS).
# CGO_ENABLED=0 creates a statically linked binary (no external C libs needed).
# The output MUST be named "bootstrap" for provided.al2023 runtime.
resource "null_resource" "lambda_build" {
  triggers = {
    source_hash = filesha256("${var.source_dir}/main.go")
  }

  provisioner "local-exec" {
    command     = "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap main.go"
    working_dir = var.source_dir
  }
}

# --------------------------------------------------------------------------
# PACKAGE AS ZIP
# --------------------------------------------------------------------------
# Lambda expects code uploaded as a ZIP file.
# This creates a ZIP containing just the "bootstrap" binary.
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "${var.source_dir}/bootstrap"
  output_path = "${var.source_dir}/lambda.zip"

  depends_on = [null_resource.lambda_build]
}

# --------------------------------------------------------------------------
# LAMBDA FUNCTION
# --------------------------------------------------------------------------
resource "aws_lambda_function" "order_processor" {
  function_name = "${var.service_name}-order-processor"
  role          = var.execution_role_arn
  handler       = "bootstrap"        # Binary name inside the ZIP
  runtime       = "provided.al2023"  # Custom runtime for Go

  filename         = data.archive_file.lambda_zip.output_path
  source_code_hash = data.archive_file.lambda_zip.output_base64sha256

  memory_size = 512   # MB (as specified in instructions)
  timeout     = 30    # seconds (must be > 3s payment processing)

  environment {
    variables = {
      # No SQS_QUEUE_URL needed! Lambda gets events directly from SNS.
      # No WORKER_COUNT needed! AWS scales Lambda instances automatically.
      ENVIRONMENT = "lambda"
    }
  }
}

# --------------------------------------------------------------------------
# SNS → LAMBDA SUBSCRIPTION
# --------------------------------------------------------------------------
# This tells SNS: "When a message arrives, invoke this Lambda function."
# Unlike SQS (which requires polling), SNS PUSHES to Lambda directly.
resource "aws_sns_topic_subscription" "lambda" {
  topic_arn = var.sns_topic_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor.arn
}

# --------------------------------------------------------------------------
# LAMBDA PERMISSION
# --------------------------------------------------------------------------
# By default, Lambda functions can only be invoked by the account owner.
# This permission allows the SNS service to invoke our Lambda function.
# Without this, SNS would get "Access Denied" when trying to trigger Lambda.
resource "aws_lambda_permission" "allow_sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = var.sns_topic_arn
}
