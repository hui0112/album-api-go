package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Modes for the downstream service
const (
	ModeNormal = "normal"
	ModeSlow   = "slow"
	ModeCrash  = "crash"
)

var (
	currentMode = ModeNormal
	modeMu      sync.RWMutex
)

func getMode() string {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return currentMode
}

func setMode(mode string) {
	modeMu.Lock()
	defer modeMu.Unlock()
	currentMode = mode
}

// /api/validate - simulates an order validation service
func validateHandler(c *gin.Context) {
	mode := getMode()
	switch mode {
	case ModeNormal:
		time.Sleep(10 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{
			"status":  "validated",
			"message": "Order is valid",
		})

	case ModeSlow:
		// Random delay between 3-5 seconds
		delay := 3000 + rand.Intn(2000)
		time.Sleep(time.Duration(delay) * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{
			"status":  "validated",
			"message": fmt.Sprintf("Order is valid (slow: %dms)", delay),
		})

	case ModeCrash:
		// 50% chance of timeout (no response), 50% chance of 500 error
		if rand.Intn(2) == 0 {
			// Simulate hang - sleep longer than any reasonable timeout
			time.Sleep(30 * time.Second)
			c.JSON(http.StatusOK, gin.H{"status": "validated"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Internal server error - database unavailable",
			})
		}
	}
}

// /control/mode?mode=normal|slow|crash - switch downstream behavior
func controlHandler(c *gin.Context) {
	mode := c.Query("mode")
	switch mode {
	case ModeNormal, ModeSlow, ModeCrash:
		setMode(mode)
		c.JSON(http.StatusOK, gin.H{
			"message":      fmt.Sprintf("Mode switched to: %s", mode),
			"current_mode": mode,
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "Invalid mode. Use: normal, slow, crash",
			"current_mode": getMode(),
		})
	}
}

// /control/status - check current mode
func statusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":      "downstream-validator",
		"current_mode": getMode(),
	})
}

func main() {
	router := gin.Default()

	router.POST("/api/validate", validateHandler)
	router.GET("/api/validate", validateHandler)
	router.GET("/control/mode", controlHandler)
	router.GET("/control/status", statusHandler)

	fmt.Println("Downstream service starting on :9090")
	fmt.Println("Modes: normal (10ms), slow (3-5s), crash (500/timeout)")
	router.Run(":9090")
}
