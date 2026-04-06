package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Result records a single request's outcome.
type Result struct {
	Timestamp   time.Time
	Operation   string // "read" or "write"
	Key         string
	Latency     time.Duration
	StatusCode  int
	Version     int
	StaleRead   bool
	TimeSinceKW time.Duration // time since last write to same key
}

// keyTracker tracks the latest known version for each key.
type keyTracker struct {
	mu       sync.Mutex
	versions map[string]int
	lastWrite map[string]time.Time
}

func (kt *keyTracker) recordWrite(key string, version int) {
	kt.mu.Lock()
	defer kt.mu.Unlock()
	kt.versions[key] = version
	kt.lastWrite[key] = time.Now()
}

func (kt *keyTracker) checkStale(key string, version int) (bool, time.Duration) {
	kt.mu.Lock()
	defer kt.mu.Unlock()
	expected, ok := kt.versions[key]
	if !ok {
		return false, 0
	}
	var sinceWrite time.Duration
	if t, ok := kt.lastWrite[key]; ok {
		sinceWrite = time.Since(t)
	}
	return version < expected, sinceWrite
}

func main() {
	// CLI flags
	writeTarget := flag.String("write-target", "http://localhost:8080", "Target URL for write requests (leader or coordinator)")
	readTargets := flag.String("read-targets", "http://localhost:8080", "Comma-separated target URLs for read requests")
	writePct := flag.Int("write-pct", 10, "Percentage of operations that are writes (0-100)")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent workers")
	duration := flag.Duration("duration", 30*time.Second, "Duration of the load test")
	keySpace := flag.Int("keys", 10, "Number of distinct keys to use")
	output := flag.String("output", "results.csv", "Output CSV file")
	flag.Parse()

	readURLs := splitComma(*readTargets)

	tracker := &keyTracker{
		versions:  make(map[string]int),
		lastWrite: make(map[string]time.Time),
	}

	var (
		results   []Result
		resultsMu sync.Mutex
		totalOps  int64
		staleOps  int64
	)

	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(*duration)

	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for time.Now().Before(deadline) {
				key := fmt.Sprintf("key-%d", rng.Intn(*keySpace))
				isWrite := rng.Intn(100) < *writePct

				var res Result
				res.Timestamp = time.Now()
				res.Key = key

				if isWrite {
					res.Operation = "write"
					value := fmt.Sprintf("v-%d-%d", workerID, time.Now().UnixMilli())

					body, _ := json.Marshal(map[string]string{"key": key, "value": value})
					start := time.Now()
					resp, err := client.Post(*writeTarget+"/set", "application/json", bytes.NewReader(body))
					res.Latency = time.Since(start)

					if err != nil {
						res.StatusCode = 0
					} else {
						res.StatusCode = resp.StatusCode
						if resp.StatusCode == http.StatusCreated {
							respBody, _ := io.ReadAll(resp.Body)
							var r struct{ Version int `json:"version"` }
							json.Unmarshal(respBody, &r)
							res.Version = r.Version
							tracker.recordWrite(key, r.Version)
						}
						resp.Body.Close()
					}
				} else {
					res.Operation = "read"
					target := readURLs[rng.Intn(len(readURLs))]

					start := time.Now()
					resp, err := client.Get(target + "/get/" + key)
					res.Latency = time.Since(start)

					if err != nil {
						res.StatusCode = 0
					} else {
						res.StatusCode = resp.StatusCode
						if resp.StatusCode == http.StatusOK {
							respBody, _ := io.ReadAll(resp.Body)
							var r struct {
								Value   string `json:"value"`
								Version int    `json:"version"`
							}
							json.Unmarshal(respBody, &r)
							res.Version = r.Version
							stale, sinceWrite := tracker.checkStale(key, r.Version)
							res.StaleRead = stale
							res.TimeSinceKW = sinceWrite
							if stale {
								atomic.AddInt64(&staleOps, 1)
							}
						}
						resp.Body.Close()
					}
				}

				atomic.AddInt64(&totalOps, 1)
				resultsMu.Lock()
				results = append(results, res)
				resultsMu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Write CSV
	f, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"timestamp", "operation", "key", "latency_ms", "status_code", "version", "stale_read", "time_since_last_write_ms"})
	for _, r := range results {
		w.Write([]string{
			r.Timestamp.Format(time.RFC3339Nano),
			r.Operation,
			r.Key,
			fmt.Sprintf("%.2f", float64(r.Latency.Microseconds())/1000.0),
			strconv.Itoa(r.StatusCode),
			strconv.Itoa(r.Version),
			strconv.FormatBool(r.StaleRead),
			fmt.Sprintf("%.2f", float64(r.TimeSinceKW.Microseconds())/1000.0),
		})
	}
	w.Flush()

	// Print summary
	total := atomic.LoadInt64(&totalOps)
	stale := atomic.LoadInt64(&staleOps)
	fmt.Printf("\n=== Load Test Summary ===\n")
	fmt.Printf("Duration:     %v\n", *duration)
	fmt.Printf("Concurrency:  %d\n", *concurrency)
	fmt.Printf("Write%%:       %d%%\n", *writePct)
	fmt.Printf("Key space:    %d\n", *keySpace)
	fmt.Printf("Total ops:    %d\n", total)
	fmt.Printf("Stale reads:  %d\n", stale)
	if total > 0 {
		fmt.Printf("Stale rate:   %.2f%%\n", float64(stale)/float64(total)*100)
	}
	fmt.Printf("Results:      %s\n", *output)

	// Latency summary
	var readLats, writeLats []float64
	for _, r := range results {
		ms := float64(r.Latency.Microseconds()) / 1000.0
		if r.Operation == "read" {
			readLats = append(readLats, ms)
		} else {
			writeLats = append(writeLats, ms)
		}
	}
	printLatencyStats("Read", readLats)
	printLatencyStats("Write", writeLats)
}

func splitComma(s string) []string {
	var result []string
	for _, part := range bytes.Split([]byte(s), []byte(",")) {
		trimmed := bytes.TrimSpace(part)
		if len(trimmed) > 0 {
			result = append(result, string(trimmed))
		}
	}
	return result
}

func printLatencyStats(label string, lats []float64) {
	if len(lats) == 0 {
		return
	}
	var sum float64
	min, max := lats[0], lats[0]
	for _, v := range lats {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	avg := sum / float64(len(lats))
	fmt.Printf("\n%s Latency (ms): count=%d avg=%.1f min=%.1f max=%.1f\n", label, len(lats), avg, min, max)
}
