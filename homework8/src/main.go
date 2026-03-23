package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// STORE INTERFACE — the "contract" both MySQL and DynamoDB must fulfill
// ============================================================================
// This is Go's way of achieving polymorphism. We define WHAT operations
// a store must support, but not HOW. mysql_store.go and dynamo_store.go
// each provide their own implementation.

type CartStore interface {
	CreateCart(customerID int) (int, error)                         // returns cart ID
	GetCart(cartID int) (*Cart, error)                              // returns cart with items
	AddItem(cartID int, productID int, quantity int) error          // add/update item
	Close() error                                                   // cleanup connections
}

// ============================================================================
// DATA STRUCTURES — shared by both implementations
// ============================================================================

type Cart struct {
	ID         int        `json:"shopping_cart_id"`
	CustomerID int        `json:"customer_id"`
	Items      []CartItem `json:"items"`
	CreatedAt  string     `json:"created_at"`
}

type CartItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

// Request bodies
type CreateCartRequest struct {
	CustomerID int `json:"customer_id" binding:"required,gt=0"`
}

type AddItemRequest struct {
	ProductID int `json:"product_id" binding:"required,gt=0"`
	Quantity  int `json:"quantity"   binding:"required,gt=0"`
}

// ============================================================================
// GLOBAL STORE — set at startup based on DB_TYPE environment variable
// ============================================================================
var store CartStore

func main() {
	// Decide which database backend to use
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "mysql" // default
	}

	log.Printf("Starting shopping cart API with DB_TYPE=%s", dbType)

	var err error
	switch dbType {
	case "mysql":
		store, err = NewMySQLStore()
	case "dynamodb":
		store, err = NewDynamoStore()
	default:
		log.Fatalf("Unknown DB_TYPE: %s (must be 'mysql' or 'dynamodb')", dbType)
	}
	if err != nil {
		log.Fatalf("Failed to initialize %s store: %v", dbType, err)
	}
	defer store.Close()

	// Set up routes
	router := gin.Default()

	router.GET("/health", healthCheck)
	router.POST("/shopping-carts", createCart)
	router.GET("/shopping-carts/:id", getCart)
	router.POST("/shopping-carts/:id/items", addItem)

	log.Println("Shopping Cart API listening on :8080")
	router.Run(":8080")
}

// ============================================================================
// HANDLERS — these call the store interface, so they work with ANY backend
// ============================================================================

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"db_type": os.Getenv("DB_TYPE"),
	})
}

// POST /shopping-carts
// Creates a new shopping cart for a customer.
func createCart(c *gin.Context) {
	var req CreateCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "customer_id is required and must be a positive integer",
		})
		return
	}

	cartID, err := store.CreateCart(req.CustomerID)
	if err != nil {
		log.Printf("Error creating cart: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "CREATE_FAILED",
			"message": "Failed to create shopping cart",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"shopping_cart_id": cartID,
	})
}

// GET /shopping-carts/:id
// Retrieves a cart with all its items.
func getCart(c *gin.Context) {
	cartID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Cart ID must be an integer",
		})
		return
	}

	cart, err := store.GetCart(cartID)
	if err != nil {
		log.Printf("Error getting cart %d: %v", cartID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "GET_FAILED",
			"message": "Failed to retrieve shopping cart",
		})
		return
	}

	if cart == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "Shopping cart not found",
		})
		return
	}

	c.JSON(http.StatusOK, cart)
}

// POST /shopping-carts/:id/items
// Adds or updates an item in the cart.
func addItem(c *gin.Context) {
	cartID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Cart ID must be an integer",
		})
		return
	}

	var req AddItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "product_id and quantity are required positive integers",
		})
		return
	}

	// First check if cart exists
	cart, err := store.GetCart(cartID)
	if err != nil {
		log.Printf("Error checking cart %d: %v", cartID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ADD_FAILED",
			"message": "Failed to add item to cart",
		})
		return
	}
	if cart == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "Shopping cart not found",
		})
		return
	}

	if err := store.AddItem(cartID, req.ProductID, req.Quantity); err != nil {
		log.Printf("Error adding item to cart %d: %v", cartID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ADD_FAILED",
			"message": "Failed to add item to cart",
		})
		return
	}

	c.Status(http.StatusNoContent) // 204 — success, no body
}
