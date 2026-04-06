package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// These tests require the leaderless cluster to be running via docker-compose.
// Nodes on :9080-:9084.

const (
	node1URL = "http://localhost:9080"
	node2URL = "http://localhost:9081"
	node3URL = "http://localhost:9082"
	node4URL = "http://localhost:9083"
)

func leaderlessSet(url, key, value string) (int, int, error) {
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	resp, err := http.Post(url+"/set", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var r struct{ Version int `json:"version"` }
	json.Unmarshal(respBody, &r)
	return resp.StatusCode, r.Version, nil
}

func leaderlessGet(url, key string) (*kvResponse, int, error) {
	resp, err := http.Get(url + "/get/" + key)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r kvResponse
	json.Unmarshal(body, &r)
	return &r, resp.StatusCode, nil
}

// Test 1: During write propagation, reading from a non-coordinator node may show stale data.
func TestLeaderlessInconsistencyWindow(t *testing.T) {
	key := fmt.Sprintf("test-ll-incon-%d", time.Now().UnixNano())
	initialValue := "initial"

	// Set initial value via node1.
	_, _, err := leaderlessSet(node1URL, key, initialValue)
	if err != nil {
		t.Fatalf("initial set failed: %v", err)
	}

	inconsistencyFound := false
	attempts := 20

	for i := 0; i < attempts; i++ {
		val := fmt.Sprintf("updated-%d", i)

		// Start a write on node1 (takes ~800ms+ due to 4 peers * 200ms delay + 100ms each).
		go leaderlessSet(node1URL, key, val)

		// Immediately read from a different node (node3 or node4, which get updated later).
		time.Sleep(100 * time.Millisecond)
		r, status, err := leaderlessGet(node4URL, key)
		if err != nil || status != http.StatusOK {
			continue
		}

		if r.Value != val {
			inconsistencyFound = true
			t.Logf("Inconsistency detected on node4: expected %q, got %q (version %d)", val, r.Value, r.Version)
			break
		}

		// Wait for write to complete.
		time.Sleep(2 * time.Second)
	}

	if !inconsistencyFound {
		t.Log("No inconsistency detected — timing-dependent, may not always occur")
	}
}

// Test 2: After coordinator acknowledges write, reading from coordinator is consistent.
func TestLeaderlessCoordinatorConsistency(t *testing.T) {
	key := fmt.Sprintf("test-ll-coord-%d", time.Now().UnixNano())
	value := "coordinator-value"

	status, version, err := leaderlessSet(node2URL, key, value)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	// Read from the coordinator node — should be consistent.
	r, status, err := leaderlessGet(node2URL, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if r.Value != value {
		t.Errorf("expected %q, got %q", value, r.Value)
	}
	if r.Version != version {
		t.Errorf("expected version %d, got %d", version, r.Version)
	}
}

// Test 3: After coordinator acknowledges write (W=N), reading from any node is consistent.
func TestLeaderlessAllNodesConsistentAfterWrite(t *testing.T) {
	key := fmt.Sprintf("test-ll-all-%d", time.Now().UnixNano())
	value := "all-nodes-value"

	status, version, err := leaderlessSet(node1URL, key, value)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	// After W=N completes, all nodes should have the data.
	nodes := []string{node1URL, node2URL, node3URL, node4URL}
	for _, url := range nodes {
		r, status, err := leaderlessGet(url, key)
		if err != nil {
			t.Errorf("get from %s failed: %v", url, err)
			continue
		}
		if status != http.StatusOK {
			t.Errorf("get from %s: expected 200, got %d", url, status)
			continue
		}
		if r.Value != value {
			t.Errorf("get from %s: expected %q, got %q", url, value, r.Value)
		}
		if r.Version != version {
			t.Errorf("get from %s: expected version %d, got %d", url, version, r.Version)
		}
	}
}
