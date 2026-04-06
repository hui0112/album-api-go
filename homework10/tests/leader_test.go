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

// These tests require the leader-follower cluster to be running via docker-compose.
// Leader on :8090, Followers on :8081-:8084.

const (
	leaderURL    = "http://localhost:8090"
	follower1URL = "http://localhost:8081"
	follower2URL = "http://localhost:8082"
)

type kvResponse struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
	Error   string `json:"error"`
}

func setKey(url, key, value string) (int, int, error) {
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

func getKey(url, key string) (*kvResponse, int, error) {
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

func localRead(url, key string) (*kvResponse, int, error) {
	resp, err := http.Get(url + "/local_read/" + key)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r kvResponse
	json.Unmarshal(body, &r)
	return &r, resp.StatusCode, nil
}

// Test 1: After leader acknowledges write, reading from leader returns consistent data.
func TestLeaderReadAfterWrite(t *testing.T) {
	key := fmt.Sprintf("test-lraw-%d", time.Now().UnixNano())
	value := "hello-leader"

	status, version, err := setKey(leaderURL, key, value)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	// Read from leader — should be consistent.
	r, status, err := getKey(leaderURL, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if r.Value != value {
		t.Errorf("expected value %q, got %q", value, r.Value)
	}
	if r.Version != version {
		t.Errorf("expected version %d, got %d", version, r.Version)
	}
}

// Test 2: After leader acknowledges write (W=5), reading from follower returns consistent data.
func TestFollowerReadAfterWrite(t *testing.T) {
	key := fmt.Sprintf("test-fraw-%d", time.Now().UnixNano())
	value := "hello-follower"

	status, version, err := setKey(leaderURL, key, value)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	// Read from follower — should be consistent after W=5 ack.
	r, status, err := getKey(follower1URL, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if r.Value != value {
		t.Errorf("expected value %q, got %q", value, r.Value)
	}
	if r.Version != version {
		t.Errorf("expected version %d, got %d", version, r.Version)
	}
}

// Test 3: During a set operation, local_read on followers might show inconsistency.
func TestLocalReadInconsistencyWindow(t *testing.T) {
	key := fmt.Sprintf("test-incon-%d", time.Now().UnixNano())
	initialValue := "initial"
	updatedValue := "updated"

	// Set initial value and wait for propagation.
	_, _, err := setKey(leaderURL, key, initialValue)
	if err != nil {
		t.Fatalf("initial set failed: %v", err)
	}

	// Now send another write (async — don't wait for response) and immediately local_read followers.
	inconsistencyFound := false
	attempts := 20

	for i := 0; i < attempts; i++ {
		val := fmt.Sprintf("%s-%d", updatedValue, i)
		go setKey(leaderURL, key, val)

		// Immediately try local_read on a follower.
		time.Sleep(50 * time.Millisecond) // small delay to let the write start
		r, status, err := localRead(follower2URL, key)
		if err != nil || status != http.StatusOK {
			continue
		}

		// If follower still has old data while write is in progress, that's inconsistency.
		if r.Value != val {
			inconsistencyFound = true
			t.Logf("Inconsistency detected: expected %q but follower has %q (version %d)", val, r.Value, r.Version)
			break
		}

		// Wait for write to complete before next iteration.
		time.Sleep(2 * time.Second)
	}

	if !inconsistencyFound {
		t.Log("No inconsistency detected in local_read — this can happen if delays are short or timing is unlucky")
	}
}
