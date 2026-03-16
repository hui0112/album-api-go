# ============================================================================
# MESSAGING MODULE (SNS + SQS)
# ============================================================================
#
# This module creates the async messaging pipeline:
#
#   Order Receiver ──publish──► SNS Topic ──deliver──► SQS Queue ──poll──► Order Processor
#
# WHY TWO SERVICES (SNS + SQS)?
# - SNS provides fan-out: one publish reaches all subscribers
# - SQS provides buffering: messages wait safely until a worker is ready
# - Together: publish once, buffer reliably, process at your own pace
#
# See CONCEPTS.md section 4 for detailed explanation.

# --------------------------------------------------------------------------
# SNS TOPIC
# --------------------------------------------------------------------------
# A "topic" is a named channel. Publishing to it delivers to all subscribers.
# Our Order Receiver publishes order JSON here.
resource "aws_sns_topic" "orders" {
  name = "${var.service_name}-order-processing-events"
}

# --------------------------------------------------------------------------
# SQS QUEUE
# --------------------------------------------------------------------------
# The queue buffers messages until workers are ready to process them.
#
# Key settings (from homework instructions):
# - visibility_timeout_seconds = 30
#     When a worker pulls a message, it becomes invisible for 30 seconds.
#     If the worker doesn't delete it within 30s (crash/timeout), the
#     message reappears for another worker to try.
#
# - message_retention_seconds = 345600 (4 days)
#     Messages that aren't processed within 4 days are automatically deleted.
#     Safety net — in practice, messages should be processed within minutes.
#
# - receive_wait_time_seconds = 20 (long polling)
#     When a worker calls ReceiveMessage, SQS waits up to 20 seconds for
#     messages to arrive before returning empty. This reduces API calls
#     and empty responses compared to short polling (0 seconds).
resource "aws_sqs_queue" "orders" {
  name                       = "${var.service_name}-order-processing-queue"
  visibility_timeout_seconds = 30
  message_retention_seconds  = 345600  # 4 days in seconds
  receive_wait_time_seconds  = 20      # Long polling
}

# --------------------------------------------------------------------------
# SQS QUEUE POLICY
# --------------------------------------------------------------------------
# By default, SQS queues only accept messages from the queue owner.
# We need to allow our SNS topic to send messages to this queue.
# This policy says: "Allow sns:SendMessage from our specific SNS topic."

# data.aws_caller_identity gives us the current AWS account ID.
# We need it to construct the queue ARN in the policy.
data "aws_caller_identity" "current" {}

resource "aws_sqs_queue_policy" "allow_sns" {
  queue_url = aws_sqs_queue.orders.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowSNSPublish"
        Effect    = "Allow"
        Principal = { Service = "sns.amazonaws.com" }
        Action    = "sqs:SendMessage"
        Resource  = aws_sqs_queue.orders.arn
        Condition = {
          ArnEquals = {
            "aws:SourceArn" = aws_sns_topic.orders.arn
          }
        }
      }
    ]
  })
}

# --------------------------------------------------------------------------
# SNS → SQS SUBSCRIPTION
# --------------------------------------------------------------------------
# This connects the SNS topic to the SQS queue.
# Every message published to the topic is automatically delivered to the queue.
#
# raw_message_delivery = false (default):
#   SNS wraps the message in an envelope with metadata (Type, TopicArn, etc.)
#   Our Order Processor needs to unwrap this envelope (see SNSMessage struct).
resource "aws_sns_topic_subscription" "sqs" {
  topic_arn = aws_sns_topic.orders.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.orders.arn
}
