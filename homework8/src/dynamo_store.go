package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ============================================================================
// DynamoStore — implements the CartStore interface using DynamoDB
// ============================================================================
//
// KEY DIFFERENCE FROM MySQL:
// - No connection pool to manage (SDK handles HTTP connections internally)
// - No schema to create (DynamoDB is schemaless — table created by Terraform)
// - No JOINs needed (cart + items stored in a single Item)
// - No transactions needed for add-item (single Item update)

type DynamoStore struct {
	client    *dynamodb.Client
	tableName string
	nextID    atomic.Int64 // Simple counter for generating cart IDs
	// DynamoDB has no AUTO_INCREMENT, so we generate IDs ourselves.
	// In production you'd use UUID or a distributed ID generator.
	// For this homework, a simple atomic counter works fine.
}

// dynamoCart is the DynamoDB Item structure.
// The struct tags tell the SDK how to convert Go types to DynamoDB attribute values.
type dynamoCart struct {
	CartID     string           `dynamodbav:"cart_id"`     // Partition Key (String in DynamoDB)
	CustomerID int              `dynamodbav:"customer_id"`
	Items      []dynamoCartItem `dynamodbav:"items"`       // Embedded list of items
	CreatedAt  string           `dynamodbav:"created_at"`
}

type dynamoCartItem struct {
	ProductID int `dynamodbav:"product_id"`
	Quantity  int `dynamodbav:"quantity"`
}

func NewDynamoStore() (*DynamoStore, error) {
	// Load AWS configuration (region, credentials from environment/IAM role)
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(os.Getenv("AWS_REGION")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)
	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		return nil, fmt.Errorf("DYNAMODB_TABLE environment variable not set")
	}

	// Seed the counter with current timestamp to avoid collisions across restarts
	store := &DynamoStore{
		client:    client,
		tableName: tableName,
	}
	store.nextID.Store(time.Now().UnixMilli() % 1000000)

	log.Printf("DynamoDB store initialized with table: %s", tableName)
	return store, nil
}

// ============================================================================
// CreateCart — PutItem: creates a new Item in the table
// ============================================================================
func (s *DynamoStore) CreateCart(customerID int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate a new cart ID， 数字转字符串存入 DynamoDB
	id := int(s.nextID.Add(1))
	cartIDStr := strconv.Itoa(id)

	cart := dynamoCart{
		CartID:     cartIDStr,
		CustomerID: customerID,
		Items:      []dynamoCartItem{}, // Empty list — no items yet
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	// MarshalMap converts the Go struct into DynamoDB attribute value format.
	// e.g., string → {"S": "value"}, int → {"N": "123"}, list → {"L": [...]}
	item, err := attributevalue.MarshalMap(cart)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal cart: %w", err)
	}

	// 创建购物车
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to put item: %w", err)
	}

	return id, nil
}

// ============================================================================
// GetCart — GetItem: retrieves a single Item by Partition Key
// ============================================================================
// This is the BIG advantage over MySQL: one call gets everything
// (cart info + all items), no JOIN needed.
func (s *DynamoStore) GetCart(cartID int) (*Cart, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 从 API 参数（int）转成字符串去查询cartID
	// strconv.Itoa（Integer to ASCII）和 strconv.Atoi（ASCII to Integer）
	// 就是 Go 里 int↔string 的转换函数。成本几乎为零。
	cartIDStr := strconv.Itoa(cartID)
	//  一次 GetItem 拿到所有数据（vs MySQL 需要 JOIN）
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartIDStr},
		},
		// Not using ConsistentRead here — we want to observe eventual consistency
		// behavior for the homework's consistency investigation (STEP II Part 3).
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	// Item not found — DynamoDB returns an empty Item map (not an error)
	if result.Item == nil {
		return nil, nil
	}

	// UnmarshalMap converts DynamoDB attribute values back into a Go struct
	var dc dynamoCart
	if err := attributevalue.UnmarshalMap(result.Item, &dc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cart: %w", err)
	}

	// Convert to the shared Cart type
	cart := &Cart{
		ID:         cartID,
		CustomerID: dc.CustomerID,
		Items:      make([]CartItem, len(dc.Items)),
		CreatedAt:  dc.CreatedAt,
	}
	for i, item := range dc.Items {
		cart.Items[i] = CartItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		}
	}

	return cart, nil
}

// ============================================================================
// AddItem — read-modify-write the items list
// ============================================================================
// Strategy: GetItem → modify items list in Go → PutItem back.
// In production you'd use UpdateExpression with list_append for atomicity,
// but for this homework the simpler approach is clearer.
func (s *DynamoStore) AddItem(cartID int, productID int, quantity int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cartIDStr := strconv.Itoa(cartID)

	// 1. Get the current cart 按分区键获取单个 Item 一次拿到购物车+所有商品
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartIDStr},
		},
		ConsistentRead: aws.Bool(true), // Strong read here to avoid stale data
	})
	if err != nil {
		return fmt.Errorf("failed to get cart: %w", err)
	}
	if result.Item == nil {
		return fmt.Errorf("cart not found")
	}

	// 2. Unmarshal and modify the items list
	var dc dynamoCart
	if err := attributevalue.UnmarshalMap(result.Item, &dc); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	// Check if item already exists (update quantity) or add new
	found := false
	for i, item := range dc.Items {
		if item.ProductID == productID {
			dc.Items[i].Quantity = quantity
			found = true
			break
		}
	}
	if !found {
		dc.Items = append(dc.Items, dynamoCartItem{
			ProductID: productID,
			Quantity:  quantity,
		})
	}

	// 3. Write back the entire cart 用于添加商品（读→改→写）
	item, err := attributevalue.MarshalMap(dc)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

// Close is a no-op for DynamoDB — the SDK manages HTTP connections internally.
func (s *DynamoStore) Close() error {
	log.Println("DynamoDB store closed (no-op)")
	return nil
}
