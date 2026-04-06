package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// KVEntry stores a value and its logical version number.
type KVEntry struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
}

// PropagateRequest is sent from the Write Coordinator to peers.
type PropagateRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int    `json:"version"`
}

// SetRequest is the client-facing write request.
type SetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Global state
var (
	store   = make(map[string]KVEntry)
	storeMu sync.RWMutex
)

// Config loaded from environment variables.
var (
	peers    []string // addresses of all other peer nodes
	nodeName string   // this node's name for logging
)

func init() {
	if p := os.Getenv("PEERS"); p != "" {
		peers = strings.Split(p, ",")
	}
	nodeName = os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "node"
	}
}

func main() {
	r := gin.Default()

	r.POST("/set", handleSet)
	r.GET("/get/:key", handleGet)
	r.GET("/local_read/:key", handleLocalRead)
	r.POST("/internal/propagate", handlePropagate)
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "node": nodeName}) })

	log.Printf("[%s] Starting leaderless node with peers=%v", nodeName, peers)
	r.Run(":8080")
}

// handleSet — any node can accept writes (becomes Write Coordinator).
// W=N: must propagate to ALL peers before responding.
func handleSet(c *gin.Context) {
	var req SetRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	// Write locally first.
	storeMu.Lock()
	entry := store[req.Key]
	entry.Value = req.Value
	entry.Version++
	store[req.Key] = entry
	localVersion := entry.Version
	storeMu.Unlock()

	// Propagate to all peers (W=N).
	ackCh := make(chan bool, len(peers))
	for _, p := range peers {
		go func(addr string) {
			ok := sendPropagate(addr, req.Key, req.Value, localVersion)
			ackCh <- ok
		}(p)
		// Coordinator sleeps 200ms after dispatching each peer.
		time.Sleep(200 * time.Millisecond)
	}

	// Wait for all acks.
	acks := 1 // self
	for range peers {
		if <-ackCh {
			acks++
		}
	}

	needed := len(peers) + 1 // W = N
	if acks >= needed {
		c.JSON(http.StatusCreated, gin.H{"version": localVersion})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not enough acks", "acks": acks, "needed": needed})
	}
}

// sendPropagate sends a propagate request to a single peer.
func sendPropagate(addr, key, value string, version int) bool {
	body, _ := json.Marshal(PropagateRequest{Key: key, Value: value, Version: version})
	resp, err := http.Post(fmt.Sprintf("http://%s/internal/propagate", addr), "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("propagate to %s failed: %v", addr, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// handlePropagate — receives data from the Write Coordinator.
func handlePropagate(c *gin.Context) {
	var req PropagateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Simulate storage delay.
	time.Sleep(100 * time.Millisecond)

	storeMu.Lock()
	existing := store[req.Key]
	if req.Version > existing.Version {
		store[req.Key] = KVEntry{Value: req.Value, Version: req.Version}
	}
	storeMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleGet — R=1: just return local value.
func handleGet(c *gin.Context) {
	key := c.Param("key")
	storeMu.RLock()
	entry, ok := store[key]
	storeMu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"value": entry.Value, "version": entry.Version})
}

// handleLocalRead — returns only this node's data (same as get for leaderless with R=1).
func handleLocalRead(c *gin.Context) {
	key := c.Param("key")
	storeMu.RLock()
	entry, ok := store[key]
	storeMu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"value": entry.Value, "version": entry.Version})
}

// fetchFromNode reads a key from a remote node (unused for R=1 but kept for completeness).
func fetchFromNode(addr, key string) (KVEntry, bool) {
	resp, err := http.Get(fmt.Sprintf("http://%s/get/%s", addr, key))
	if err != nil {
		return KVEntry{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return KVEntry{}, false
	}
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return KVEntry{}, false
	}
	return KVEntry{Value: result.Value, Version: result.Version}, true
}
