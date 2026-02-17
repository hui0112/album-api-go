package main

import (
	"net/http"
	"strconv" //need strconv.Atoi() ("ASCII to integer") to convert "42" → 42

	"github.com/gin-gonic/gin"
)

// Product represents the product schema from api.yaml
// The backtick tags like `json:"product_id"`
//   tell Go's JSON encoder to use product_id in the JSON output instead of ProductID. Without these, the JSON would have
//   Go-style names like ProductID which doesn't match the spec
type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

// In-memory storage: map of productId -> Product
var products = map[int]Product{}

func main() {
	router := gin.Default()
	router.GET("/products/:productId", getProduct)
	// Register POST endpoint — note the path includes /details after the productId
	router.POST("/products/:productId/details", addProductDetails)

	router.Run(":8080")
}

// getProduct retrieves a product by its ID from the URL path
func getProduct(c *gin.Context) {
	// Step 1: Parse the productId from the URL path string into an integer
	idStr := c.Param("productId")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
        // c.JSON() is more compact and faster, which matters for production/load testing
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Product ID must be a positive integer",
		})
		return
	}

	// Step 2: Look up the product in our map
	product, exists := products[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "Product not found",
		})
		return
	}

	// Step 3: Return the product as JSON
	c.JSON(http.StatusOK, product)
}

// addProductDetails handles POST /products/:productId/details
// Creates or updates a product's details. Returns 204 on success, 400 on bad input.
func addProductDetails(c *gin.Context) {
	// --- Step 1: Validate the productId from the URL path (same logic as GET) ---
	idStr := c.Param("productId")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Product ID must be a positive integer",
		})
		return
	}

	// --- Step 2: Parse the JSON request body into a Product struct ---
	// ShouldBindJSON reads the body and fills struct fields by matching json tags
	// We use ShouldBindJSON (not BindJSON) because BindJSON auto-writes a 400 response,
	// taking control away from us. ShouldBindJSON just returns the error and lets US
	// decide the response format (so we can match the Error schema from api.yaml)
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid JSON in request body",
			"details": err.Error(), // include what went wrong so the caller can fix it
		})
		return
	}

	// --- Step 3: Validate required fields manually ---
	// ShouldBindJSON only checks JSON syntax, not our business rules from api.yaml:
	//   - sku and manufacturer must be non-empty strings (minLength: 1)
	//   - category_id and some_other_id must be >= 1 (minimum: 1)
	//   - weight must be >= 0 (minimum: 0)
	if product.SKU == "" || product.Manufacturer == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "sku and manufacturer are required and cannot be empty",
		})
		return
	}
	if product.CategoryID < 1 || product.SomeOtherID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "category_id and some_other_id must be positive integers",
		})
		return
	}
	if product.Weight < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "weight must be non-negative",
		})
		return
	}

	// --- Step 4: Store the product ---
	// Override product_id with the one from the URL path (URL is the source of truth)
	// This way even if the body says product_id: 99, the URL's :productId wins
	product.ProductID = id

	// Save to our map — if the key already exists, it gets overwritten (update)
	// If it's new, it gets created. Either way, this is an "upsert" (update or insert)
	products[id] = product

	// 204 No Content = "success, but nothing to send back in the body"
	// This is the standard HTTP response for create/update operations that don't return data
	c.Status(http.StatusNoContent)
}
