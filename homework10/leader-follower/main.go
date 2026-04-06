package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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

// ReplicateRequest is sent from the Leader to Followers.
type ReplicateRequest struct {
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
	role      string   // "leader" or "follower"
	followers []string // addresses of follower nodes (leader only)
	wValue    int      // W quorum size
	rValue    int      // R quorum size
	nValue    int      // total nodes (always 5)
)

func init() {
	role = os.Getenv("ROLE")
	if role == "" {
		role = "leader"
	}

	if f := os.Getenv("FOLLOWERS"); f != "" {
		followers = strings.Split(f, ",")
	}

	wValue = envInt("W_VALUE", 5)
	rValue = envInt("R_VALUE", 1)
	nValue = envInt("N_VALUE", 5)
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}

func main() {
	r := gin.Default()

	r.POST("/set", handleSet)
	r.GET("/get/:key", handleGet)
	r.GET("/local_read/:key", handleLocalRead)
	r.POST("/internal/replicate", handleReplicate)
	r.GET("/internal/read/:key", handleInternalRead)
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "role": role}) })

	log.Printf("[%s] Starting with W=%d R=%d N=%d followers=%v", role, wValue, rValue, nValue, followers)
	r.Run(":8080")
}

// handleSet — only the Leader accepts client writes.
func handleSet(c *gin.Context) {
	if role != "leader" {
		c.JSON(http.StatusForbidden, gin.H{"error": "writes must go to leader"})
		return
	}

	var req SetRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	// Write locally first — always counts as 1 ack.
	storeMu.Lock()
	entry := store[req.Key]
	entry.Value = req.Value
	entry.Version++
	store[req.Key] = entry
	localVersion := entry.Version
	storeMu.Unlock()

	// Replicate to followers.
	acks := 1 // leader itself
	needed := wValue

	if needed <= 1 {
		// W=1: respond immediately, replicate async.
		c.JSON(http.StatusCreated, gin.H{"version": localVersion})
		go replicateToFollowers(req.Key, req.Value, localVersion)
		return
	}

	// W>1: replicate synchronously until we have enough acks.
	ackCh := make(chan bool, len(followers))
	for _, f := range followers {
		go func(addr string) {
			ok := sendReplicate(addr, req.Key, req.Value, localVersion)
			ackCh <- ok
		}(f)
		// Leader sleeps 200ms after dispatching each follower request.
		time.Sleep(200 * time.Millisecond)
	}

	// Collect acks.
	for range followers {
		if <-ackCh {
			acks++
		}
	}

	if acks >= needed {
		c.JSON(http.StatusCreated, gin.H{"version": localVersion})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not enough acks", "acks": acks, "needed": needed})
	}
}

// replicateToFollowers sends the update to all followers (fire-and-forget for W=1).
func replicateToFollowers(key, value string, version int) {
	for _, f := range followers {
		sendReplicate(f, key, value, version)
		time.Sleep(200 * time.Millisecond)
	}
}

// sendReplicate sends a replicate request to a single follower.
func sendReplicate(addr, key, value string, version int) bool {
	body, _ := json.Marshal(ReplicateRequest{Key: key, Value: value, Version: version})
	resp, err := http.Post(fmt.Sprintf("http://%s/internal/replicate", addr), "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("replicate to %s failed: %v", addr, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// handleReplicate — Follower receives data from Leader.
func handleReplicate(c *gin.Context) {
	var req ReplicateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Simulate storage delay.
	time.Sleep(100 * time.Millisecond)

	storeMu.Lock()
	existing := store[req.Key]
	// Only apply if the incoming version is newer.
	if req.Version > existing.Version {
		store[req.Key] = KVEntry{Value: req.Value, Version: req.Version}
	}
	storeMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleGet — client-facing read. Behaviour depends on R value.
func handleGet(c *gin.Context) {
	key := c.Param("key")

	if rValue <= 1 {
		// R=1: just read local.
		storeMu.RLock()
		entry, ok := store[key]
		storeMu.RUnlock()
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"value": entry.Value, "version": entry.Version})
		return
	}

	// R>1: read from multiple nodes and return the latest version.
	type readResult struct {
		entry KVEntry
		ok    bool
	}

	results := make(chan readResult, len(followers)+1)

	// Read local.
	storeMu.RLock()
	localEntry, localOk := store[key]
	storeMu.RUnlock()
	results <- readResult{entry: localEntry, ok: localOk}

	// Read from followers.
	for _, f := range followers {
		go func(addr string) {
			entry, ok := fetchFromNode(addr, key)
			results <- readResult{entry: entry, ok: ok}
		}(f)
	}

	// Collect R results (leader + followers).
	best := KVEntry{Version: -1}
	found := false
	collected := 0
	needed := rValue

	for collected < len(followers)+1 {
		r := <-results
		collected++
		if r.ok && r.entry.Version > best.Version {
			best = r.entry
			found = true
		}
		// Once we have enough reads with at least one found, we can return.
		if collected >= needed && found {
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"value": best.Value, "version": best.Version})
}

// handleLocalRead — returns only the local node's data (test endpoint).
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

// handleInternalRead — Leader queries Followers for quorum reads.
func handleInternalRead(c *gin.Context) {
	key := c.Param("key")

	// Simulate read delay on follower.
	time.Sleep(50 * time.Millisecond)

	storeMu.RLock()
	entry, ok := store[key]
	storeMu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"value": entry.Value, "version": entry.Version})
}

// fetchFromNode reads a key from a remote node via the internal read endpoint.
func fetchFromNode(addr, key string) (KVEntry, bool) {
	resp, err := http.Get(fmt.Sprintf("http://%s/internal/read/%s", addr, key))
	if err != nil {
		log.Printf("read from %s failed: %v", addr, err)
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
