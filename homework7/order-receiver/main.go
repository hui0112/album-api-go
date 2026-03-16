package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// Item represents a single item in an order.
type Item struct {
	ProductID int    `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Price     float64 `json:"price"`
}

// Order represents a customer order as defined in the homework spec.
// JSON tags control how Go marshals/unmarshals this struct to/from JSON.
type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"` // pending, processing, completed
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// ============================================================================
// GLOBAL STATE
// ============================================================================

// snsClient is our connection to AWS SNS. Initialized once at startup.
var snsClient *sns.Client

// snsTopicARN is the Amazon Resource Name of our SNS topic.
// Set via environment variable SNS_TOPIC_ARN (injected by Terraform).
var snsTopicARN string

// paymentSlots is a buffered channel acting as a SEMAPHORE.
//
// WHY A BUFFERED CHANNEL?
// The homework says: "when a Go routine sleeps, the thread is not blocked."
// time.Sleep() only pauses the goroutine — the OS thread runs other goroutines.
// So if 100 requests arrive, 100 goroutines all sleep concurrently = no bottleneck!
//
// To simulate a REAL payment processor that can only handle N concurrent requests,
// we use a buffered channel with capacity N. Sending to a full channel BLOCKS,
// creating the queuing behavior we need.
//
// Capacity 5: simulates a payment processor handling 5 concurrent payments.
// Each payment takes 3 seconds, so throughput = 5 / 3 ≈ 1.67 orders/second.
// (This is the bottleneck the homework wants us to discover!)
var paymentSlots = make(chan struct{}, 5)

// orderCount tracks total orders processed (for logging/debugging).
var orderCount atomic.Int64

// ============================================================================
// MAIN - Application Entry Point
// ============================================================================

func main() {
	// Read the SNS topic ARN from environment variable.
	// Terraform injects this into the ECS task definition.
	snsTopicARN = os.Getenv("SNS_TOPIC_ARN")

	// Initialize AWS SDK. It auto-detects region from ECS task metadata.
	// Locally, it reads from ~/.aws/config or AWS_REGION env var.
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("WARNING: Could not load AWS config: %v", err)
		log.Println("SNS publishing will fail. This is OK for local sync testing.")
	} else {
		snsClient = sns.NewFromConfig(cfg)
		log.Printf("AWS SNS client initialized. Topic ARN: %s", snsTopicARN)
	}

	// Set up Gin router with default middleware (logger + recovery).
	router := gin.Default()

	// Register endpoints:
	// - /health: ALB health check (required by ECS/ALB target group)
	// - /orders/sync: Phase 1 — synchronous payment processing
	// - /orders/async: Phase 3 — async processing via SNS
	router.GET("/health", healthCheck)
	router.POST("/orders/sync", handleSyncOrder)
	router.POST("/orders/async", handleAsyncOrder)

	log.Println("Order Receiver starting on :8080")
	router.Run(":8080")
}

// ============================================================================
// HANDLERS
// ============================================================================

// healthCheck returns 200 OK so ALB knows this task is healthy.
// ALB checks this endpoint every 30 seconds. If it fails 3 times in a row,
// ALB stops routing traffic to this task.
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// handleSyncOrder processes an order SYNCHRONOUSLY.
//
// Flow: Customer → API → Payment (3s blocking) → Response
//
// This is the Phase 1 implementation. Under flash sale load (60 orders/second),
// this endpoint will bottleneck because paymentSlots only allows 5 concurrent
// payments, each taking 3 seconds.
func handleSyncOrder(c *gin.Context) {
	// Step 1: Parse the JSON request body into an Order struct.
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid JSON body",
		})
		return
	}

	// Step 2: Generate a unique order ID and set initial status.
	order.OrderID = uuid.New().String()
	order.Status = "pending"
	order.CreatedAt = time.Now()

	// Step 3: Simulate synchronous payment processing.
	// This is where the BOTTLENECK happens.
	//
	// paymentSlots <- struct{}{} tries to put a token into the channel.
	// If the channel is full (5 payments already running), this BLOCKS.
	// The goroutine (and therefore the HTTP request) WAITS here.
	//
	// This is exactly what happens with a real payment processor that
	// has limited concurrent connections.
	order.Status = "processing"
	paymentSlots <- struct{}{} // Acquire a payment slot (BLOCKS if full)
	time.Sleep(3 * time.Second) // Simulate 3-second payment verification
	<-paymentSlots              // Release the payment slot

	// Step 4: Payment complete, return success.
	order.Status = "completed"
	count := orderCount.Add(1)
	log.Printf("SYNC order #%d completed: %s", count, order.OrderID)

	c.JSON(http.StatusOK, gin.H{
		"order":   order,
		"message": "Order processed synchronously",
	})
}

// handleAsyncOrder processes an order ASYNCHRONOUSLY via SNS.
//
// Flow: Customer → API → Publish to SNS → Return 202 Accepted (<100ms)
//       (Background: SNS → SQS → Order Processor → Payment)
//
// This is the Phase 3 implementation. The customer gets an immediate response.
// The actual payment processing happens later in the Order Processor service.
func handleAsyncOrder(c *gin.Context) {
	// Step 1: Parse the JSON request body.
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid JSON body",
		})
		return
	}

	// Step 2: Generate order ID and set status to "pending".
	// Unlike sync, we DON'T wait for payment. Status stays "pending".
	order.OrderID = uuid.New().String()
	order.Status = "pending"
	order.CreatedAt = time.Now()

	// Step 3: Check that SNS is configured.
	if snsClient == nil || snsTopicARN == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SNS_NOT_CONFIGURED",
			"message": "Async processing not available (SNS not configured)",
		})
		return
	}

	// Step 4: Serialize the order to JSON for the SNS message body.
	orderJSON, err := json.Marshal(order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "SERIALIZATION_ERROR",
			"message": "Failed to serialize order",
		})
		return
	}

	// Step 5: Publish the order to SNS.
	//
	// sns.Publish() sends the message to the SNS topic. SNS then delivers
	// it to all subscribers (in our case, just the SQS queue).
	//
	// This is FAST (<100ms) because we're just handing off to AWS,
	// not doing any payment processing.
	_, err = snsClient.Publish(context.TODO(), &sns.PublishInput{
		TopicArn: aws.String(snsTopicARN),
		Message:  aws.String(string(orderJSON)),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "SNS_PUBLISH_ERROR",
			"message": fmt.Sprintf("Failed to publish order: %v", err),
		})
		return
	}

	// Step 6: Return 202 Accepted immediately.
	// 202 means "we received our request and will process it later."
	// The customer doesn't wait for payment — they get a response in <100ms.
	count := orderCount.Add(1)
	log.Printf("ASYNC order #%d accepted: %s", count, order.OrderID)

	c.JSON(http.StatusAccepted, gin.H{
		"order":   order,
		"message": "Order accepted for async processing",
	})
}
