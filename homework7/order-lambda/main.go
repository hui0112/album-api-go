package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// Item and Order mirror the Order Receiver's structs.
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

// ============================================================================
// LAMBDA HANDLER
// ============================================================================
//
// HOW THIS DIFFERS FROM THE ECS ORDER PROCESSOR:
//
// ECS Processor:
//   - Runs forever in a loop, polling SQS for messages
//   - YOU manage scaling (WORKER_COUNT), health checks, infrastructure
//   - Messages go: SNS → SQS → your polling loop → process
//
// Lambda:
//   - AWS invokes this function ONCE per SNS message (or batch)
//   - AWS manages scaling automatically (spins up more Lambdas as needed)
//   - Messages go: SNS → Lambda (direct, no SQS!)
//   - No polling, no health checks, no infrastructure to manage
//
// COLD START vs WARM START:
//   - Cold start: First invocation (or after ~5 min idle), AWS must:
//     1. Download your code
//     2. Start a new execution environment
//     3. Initialize your Go binary (Init Duration in CloudWatch)
//   - Warm start: Reuses existing environment, skips initialization
//   - For 3-second payment processing, cold start overhead (~73ms) is negligible

func handler(ctx context.Context, snsEvent events.SNSEvent) error {
	// SNS delivers messages in Records[]. Usually 1 record per invocation,
	// but we loop just in case.
	for _, record := range snsEvent.Records {
		// The SNS message body IS our order JSON directly.
		// Unlike SQS (which wraps SNS in another envelope), Lambda's SNS
		// integration gives us the SNS Message field directly.
		snsMessage := record.SNS.Message

		// Parse the order
		var order Order
		if err := json.Unmarshal([]byte(snsMessage), &order); err != nil {
			log.Printf("ERROR parsing order: %v", err)
			continue
		}

		log.Printf("Processing order %s for customer %d...", order.OrderID, order.CustomerID)

		// Same 3-second payment simulation as ECS processor.
		// The difference: the customer already got 202 Accepted from the API.
		// And AWS automatically scales Lambda instances to handle concurrent orders.
		time.Sleep(3 * time.Second)

		log.Printf("COMPLETED order %s (customer %d)", order.OrderID, order.CustomerID)
	}
	return nil
}

func main() {
	// lambda.Start registers our handler and blocks forever,
	// waiting for AWS to invoke it with SNS events.
	lambda.Start(handler)
}
