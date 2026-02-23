package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Product represents a searchable product.
// JSON tags (e.g. `json:"id"`) control how Go marshals this struct into JSON.
// Without tags, the JSON keys would be Go-style "ID", "Name", etc.
type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

// We use sync.Map instead of a regular map because:
//   - Regular maps are NOT safe for concurrent read/write in Go
//   - sync.Map handles locking internally, so multiple goroutines (HTTP handlers)
//     can read from it simultaneously without data races
//   - For a read-heavy workload like search, sync.Map is more efficient than
//     wrapping a regular map with sync.RWMutex
var products sync.Map

// Sample data arrays — we rotate through these using modulo (i % len)
// to get consistent, predictable data for testing
var brands = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta", "Theta"}
var categories = []string{"Electronics", "Books", "Home", "Sports", "Clothing", "Toys", "Food", "Health"}
var descriptions = []string{
	"High quality product for everyday use",
	"Premium grade with extended warranty",
	"Budget friendly option with great value",
	"Professional series for demanding users",
	"Eco-friendly and sustainably sourced",
	"Limited edition collector's item",
	"Best seller in its category",
	"New arrival with innovative features",
}

// generateProducts creates 100,000 products and stores them in the sync.Map.
// Called once at startup — the large dataset simulates realistic memory usage,
// even though each search only checks 100 of them.
func generateProducts() {
	for i := 1; i <= 100000; i++ {
		brand := brands[i%len(brands)]
		category := categories[i%len(categories)]
		description := descriptions[i%len(descriptions)]

		p := Product{
			ID:          i,
			Name:        fmt.Sprintf("Product %s %d", brand, i),
			Category:    category,
			Description: description,
			Brand:       brand,
		}
		products.Store(i, p)
	}
}

// searchProducts checks exactly 2,000 products and returns up to 20 matches.
//
// Why 2,000? This simulates a fixed-cost computation (like running an AI model
// or processing video frames). The point is that each request takes roughly the
// same amount of CPU time regardless of the query, making CPU the bottleneck
// under load — which is what we want to demonstrate for scaling.
// Tuned so that 5 users → ~60% CPU, 20 users → ~100% CPU on 0.25 vCPU.
//
// Key: `checked` counts EVERY product examined, not just matches.
// This ensures consistent CPU cost per request.
func searchProducts(query string) ([]Product, int) {
	query = strings.ToLower(query)
	var results []Product
	totalFound := 0
	checked := 0

	products.Range(func(key, value interface{}) bool {
		// Count every product checked, not just matches
		checked++

		p := value.(Product)
		nameLower := strings.ToLower(p.Name)
		categoryLower := strings.ToLower(p.Category)
		// check if the product match
		if strings.Contains(nameLower, query) || strings.Contains(categoryLower, query) {
			totalFound++
			if len(results) < 20 {
				results = append(results, p)
			}
		}

		// Return false to stop iteration after exactly 2,000 products
		return checked < 2000
	})

	return results, totalFound
}

// healthCheck handles GET /health
// Used by ALB target group to determine if this instance can receive traffic.
// Returns 200 = healthy, any other status = unhealthy (ALB stops routing to it).
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// handleSearch handles GET /products/search?q={query}
func handleSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Query parameter 'q' is required",
		})
		return
	}

	start := time.Now()
	results, totalFound := searchProducts(query)
	elapsed := time.Since(start)

	c.JSON(http.StatusOK, gin.H{
		"products":    results,
		"total_found": totalFound,
		"search_time": elapsed.String(),
	})
}

func main() {
	generateProducts()
	fmt.Println("Generated 100,000 products")

	router := gin.Default()
	router.GET("/health", healthCheck)
	router.GET("/products/search", handleSearch)
	router.Run(":8080")
}
