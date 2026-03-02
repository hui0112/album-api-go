package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Resilience components (global)
var (
	circuitBreaker *CircuitBreaker
	bulkhead       *Bulkhead
	failFast       *FailFast
	resilienceOn   bool
	downstreamURL  string
)

func initResilience() {
	// Circuit Breaker: open after 5 failures, cooldown 10s, need 3 successes to close
	circuitBreaker = NewCircuitBreaker(5, 10*time.Second, 3)
	// Bulkhead: max 10 concurrent requests to downstream
	bulkhead = NewBulkhead(10)
	// Fail Fast: health gate
	failFast = NewFailFast()

	resilienceOn = os.Getenv("RESILIENCE_ENABLED") == "true"

	downstreamURL = os.Getenv("DOWNSTREAM_URL")
	if downstreamURL == "" {
		downstreamURL = "http://localhost:9090"
	}

	fmt.Printf("Resilience enabled: %v\n", resilienceOn)
	fmt.Printf("Downstream URL: %s\n", downstreamURL)
}

func main() {
	initResilience()

	router := gin.Default()

	// Existing album endpoints
	router.GET("/albums", getAlbums)
	router.POST("/albums", postAlbums)
	router.GET("/albums/:id", getAlbumByID)

	// New endpoints for crash & recovery demo
	router.POST("/orders", createOrder)
	router.GET("/status", getStatus)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router.Run(":" + port)
}

// album represents data about a record album.
type album struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

// albums slice to seed record album data.
var albums = []album{
	{ID: "1", Title: "Blue Train", Artist: "John Coltrane", Price: 56.99},
	{ID: "2", Title: "Jeru", Artist: "Gerry Mulligan", Price: 17.99},
	{ID: "3", Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Price: 39.99},
}
var mu sync.Mutex

// getAlbums responds with the list of all albums as JSON.
func getAlbums(c *gin.Context) {
	time.Sleep(10 * time.Millisecond)
	c.IndentedJSON(http.StatusOK, albums)
}

// postAlbums adds an album from JSON received in the request body.
func postAlbums(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	time.Sleep(50 * time.Millisecond)
	var newAlbum album

	if err := c.BindJSON(&newAlbum); err != nil {
		return
	}

	albums = append(albums, newAlbum)
	c.IndentedJSON(http.StatusCreated, newAlbum)
}

// getAlbumByID locates the album whose ID value matches the id parameter.
func getAlbumByID(c *gin.Context) {
	id := c.Param("id")

	for _, a := range albums {
		if a.ID == id {
			c.IndentedJSON(http.StatusOK, a)
			return
		}
	}
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
}

// ============================================================
// Order endpoint - calls downstream validation service
// ============================================================

type order struct {
	ID       string  `json:"id"`
	Item     string  `json:"item"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

func createOrder(c *gin.Context) {
	var newOrder order
	if err := c.BindJSON(&newOrder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order data"})
		return
	}

	if resilienceOn {
		createOrderWithResilience(c, newOrder)
	} else {
		createOrderDirect(c, newOrder)
	}
}

// createOrderDirect calls downstream with NO protection
func createOrderDirect(c *gin.Context, o order) {
	err := callDownstream()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "downstream validation failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":     "order_created",
		"order":      o,
		"resilience": false,
	})
}

// createOrderWithResilience applies Fail Fast -> Bulkhead -> Circuit Breaker
func createOrderWithResilience(c *gin.Context, o order) {
	// 1. Fail Fast: check if downstream is marked unhealthy
	if !failFast.IsHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "fail_fast: downstream is unhealthy",
			"pattern": "Fail Fast",
		})
		return
	}

	// 2. Bulkhead: check concurrency limit
	if !bulkhead.Acquire() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "bulkhead: too many concurrent requests",
			"pattern": "Bulkhead",
		})
		return
	}
	defer bulkhead.Release()

	// 3. Circuit Breaker: check if circuit is open
	if !circuitBreaker.AllowRequest() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "circuit_breaker: circuit is OPEN",
			"pattern": "Circuit Breaker",
		})
		return
	}

	// 4. Call downstream
	err := callDownstream()
	if err != nil {
		circuitBreaker.RecordFailure()

		// If circuit just opened, mark downstream unhealthy for fail fast
		if circuitBreaker.Status()["state"] == "OPEN" {
			failFast.SetHealthy(false)

			// Re-enable fail fast after cooldown
			go func() {
				time.Sleep(10 * time.Second)
				failFast.SetHealthy(true)
			}()
		}

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "downstream call failed",
			"details": err.Error(),
		})
		return
	}

	circuitBreaker.RecordSuccess()

	c.JSON(http.StatusCreated, gin.H{
		"status":     "order_created",
		"order":      o,
		"resilience": true,
	})
}

// callDownstream makes an HTTP call to the downstream validation service
func callDownstream() error {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Post(downstreamURL+"/api/validate", "application/json", nil)
	if err != nil {
		return fmt.Errorf("downstream unreachable: %v", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downstream returned status %d", resp.StatusCode)
	}

	return nil
}

// ============================================================
// Status endpoint - shows resilience component states
// ============================================================

func getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"resilience_enabled": resilienceOn,
		"circuit_breaker":    circuitBreaker.Status(),
		"bulkhead":           bulkhead.Status(),
		"fail_fast":          failFast.Status(),
		"downstream_url":     downstreamURL,
	})
}
