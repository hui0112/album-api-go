package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func main() {
	// The number of ping-pong round trips (1 million times)
	const limit = 1000000

	fmt.Println("Starting Context Switching Experiment...")

	// Experiment 1: Single Core (GOMAXPROCS = 1)
	// We force Go to use ONLY ONE OS thread.
	// This means the two goroutines must share the same CPU core.
	// When one pauses, the OS must "switch context" to the other on the same core.

	runtime.GOMAXPROCS(1) // Limit to 1 CPU
	start := time.Now()
	runPingPong(limit)
	durationSingle := time.Since(start)

	fmt.Printf("Single Core Time (GOMAXPROCS=1): %v\n", durationSingle)

	// Experiment 2: Multi Core (GOMAXPROCS = Default)
	// We allow Go to use all available CPU cores.
	// The two goroutines might run on DIFFERENT CPU cores in parallel.

	runtime.GOMAXPROCS(0) // 0 means "use default" (all available cores)
	start = time.Now()
	runPingPong(limit)
	durationMulti := time.Since(start)

	fmt.Printf("Multi Core Time  (GOMAXPROCS=0): %v\n", durationMulti)
}

// runPingPong creates two goroutines that pass a message back and forth
func runPingPong(limit int) {
	// Unbuffered channel: requires the sender and receiver to touch hands
	// This forces synchronization every single time.
	ch := make(chan int)
	var wg sync.WaitGroup

	wg.Add(2)

	// Goroutine 1: The "Ping" player
	go func() {
		defer wg.Done()
		for i := 0; i < limit; i++ {
			// Send the ball
			ch <- i
			// Wait to receive the ball back
			<-ch
		}
	}()

	// Goroutine 2: The "Pong" player
	go func() {
		defer wg.Done()
		for i := 0; i < limit; i++ {
			// Wait to receive the ball
			<-ch
			// Send the ball back
			ch <- i
		}
	}()

	wg.Wait()
}