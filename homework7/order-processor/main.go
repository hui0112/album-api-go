package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// Item and Order mirror the Order Receiver's structs so we can deserialize
// the JSON messages that come through SNS → SQS.
type Item struct {
	ProductID int     `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// SNSMessage is the envelope that SNS wraps around our order JSON.
// When SNS delivers to SQS, the SQS message body is NOT the raw order JSON.
// Instead, it's an SNS notification envelope that CONTAINS our order JSON
// in the "Message" field.
//
// Example SQS message body:
//
//	{
//	  "Type": "Notification",
//	  "Message": "{\"order_id\":\"abc\",\"customer_id\":123,...}",  <-- our order (escaped JSON string)
//	  "TopicArn": "arn:aws:sns:us-east-1:123:order-processing-events",
//	  ...
//	}
type SNSMessage struct {
	Type    string `json:"Type"`
	Message string `json:"Message"` // This contains our actual order JSON as a string
}

// ============================================================================
// GLOBAL STATE
// ============================================================================

var sqsClient *sqs.Client
var sqsQueueURL string

// processedCount tracks how many orders we've completed (for logging).
var processedCount atomic.Int64

// workerCount controls how many goroutines process orders concurrently.
// This is the knob you turn in Phase 5 to find the right balance.
var workerCount int

// ============================================================================
// MAIN
// ============================================================================

func main() {
	// Read configuration from environment variables (set by Terraform).
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL")
	if sqsQueueURL == "" {
		log.Fatal("SQS_QUEUE_URL environment variable is required")
	}

	// WORKER_COUNT controls concurrency. Default is 1 (Phase 3).
	// Phase 5 asks you to test with 5, 20, 100.
	workerCount = 1
	if wc := os.Getenv("WORKER_COUNT"); wc != "" {
		parsed, err := strconv.Atoi(wc)
		if err == nil && parsed > 0 {
			workerCount = parsed
		}
	}

	// Initialize AWS SDK.
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	sqsClient = sqs.NewFromConfig(cfg)

	log.Printf("Order Processor starting with %d worker(s)", workerCount)
	log.Printf("SQS Queue URL: %s", sqsQueueURL)

	// Start the health check HTTP server in a background goroutine.
	// ECS needs a /health endpoint to know the task is running.
	// This runs on port 8080 alongside the SQS polling.
	go startHealthServer()

	// Start the SQS polling loop. This runs forever.
	// It's the "main loop" of the processor.
	pollSQS()
}

// ============================================================================
// HEALTH CHECK SERVER
// ============================================================================

// startHealthServer runs a minimal HTTP server just for health checks.
// Even though this service doesn't serve API traffic, ECS requires a
// health endpoint to monitor task health.
func startHealthServer() {
	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "healthy",
			"workers":    workerCount,
			"processed":  processedCount.Load(),
		})
	})
	router.Run(":8080")
}

// ============================================================================
// SQS POLLING轮询 LOOP
// ============================================================================

// pollSQS continuously polls the SQS queue for new messages.
//
// THE POLLING PATTERN:
// 1. Call ReceiveMessage with WaitTimeSeconds=20 (long polling)
//    - SQS holds the connection for up to 20s waiting for messages
//    - If messages arrive during this time, they're returned immediately
//    - If no messages after 20s, returns empty (we loop and try again)
// 2. For each message received, spawn a goroutine to process it
// 3. Use a semaphore (buffered channel) to limit concurrent workers
// 4. Repeat forever
//
// WHY A SEMAPHORE限流器 HERE?
// Without it, if we receive 10 messages per poll and poll completes instantly,
// we could spawn hundreds of goroutines. The semaphore ensures at most
// `workerCount` goroutines process payments concurrently.
func pollSQS() {
	// Semaphore: limits concurrent processing goroutines.
	// This is the same buffered channel pattern as the Order Receiver,
	// but here we control it with WORKER_COUNT.
	sem := make(chan struct{}, workerCount)

	for {
		// Step 1: Receive up to 10 messages from SQS.
		// MaxNumberOfMessages=10: SQS can return at most 10 per call.
		// WaitTimeSeconds=20: Long polling — wait up to 20s for messages.
		output, err := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			QueueUrl:            &sqsQueueURL,
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			log.Printf("ERROR receiving from SQS: %v", err)
			time.Sleep(5 * time.Second) // Back off on error
			continue
		}

		// Step 2: Process each message in a goroutine.
		for _, msg := range output.Messages {
			// Acquire a worker slot (blocks if all workers are busy).
			sem <- struct{}{}

			// Spawn a goroutine for this message.
			// We pass msg by value so each goroutine has its own copy.
			go func(m types.Message) {
				defer func() { <-sem }() // Release worker slot when done
				processMessage(m)
			}(msg)
		}
	}
}

// processMessage handles a single SQS message.
//
// Steps:
// 1. Unwrap the SNS envelope to get the raw order JSON
// 2. Deserialize the order
// 3. Simulate 3-second payment processing
// 4. Delete the message from SQS (acknowledging completion)
//
// WHY DELETE AFTER PROCESSING?
// If we don't delete the message, it reappears after the visibility timeout (30s).
// This is SQS's "at-least-once" delivery guarantee. By deleting after success,
// we prevent duplicate processing. If the goroutine crashes before deleting,
// the message reappears and another worker retries it.
func processMessage(msg types.Message) {
	// Step 1: Parse the SNS envelope.
	// The SQS message body is NOT our order directly.
	// SNS wraps it: {"Type":"Notification","Message":"{our order JSON}"}
	var snsMsg SNSMessage
	if err := json.Unmarshal([]byte(*msg.Body), &snsMsg); err != nil {
		log.Printf("ERROR parsing SNS envelope: %v", err)
		// Don't delete — let it retry or go to dead letter queue
		return
	}

	// Step 2: Parse the actual order from the SNS Message field.
	var order Order
	if err := json.Unmarshal([]byte(snsMsg.Message), &order); err != nil {
		log.Printf("ERROR parsing order: %v", err)
		return
	}

	// Step 3: Simulate payment processing (3 seconds).
	// This is the same delay as the sync endpoint.
	// The difference is: the customer isn't waiting!
	log.Printf("Processing order %s for customer %d...", order.OrderID, order.CustomerID)
	time.Sleep(3 * time.Second)

	// Step 4: Delete the message from SQS.
	// ReceiptHandle is SQS's way of identifying THIS specific receipt of
	// THIS message. We need it to delete (acknowledge) the message.
	_, err := sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
		QueueUrl:      &sqsQueueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		log.Printf("ERROR deleting message: %v", err)
		return
	}

	count := processedCount.Add(1)
	log.Printf("COMPLETED order #%d: %s (customer %d)", count, order.OrderID, order.CustomerID)
}
