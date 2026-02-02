package main

import(
	"fmt"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	//1, Initialize map and add a mutex
	var mu sync.Mutex
	m := make(map[int]int)
	//2, spawn 50 goroutines
	start := time.Now()
	for g := 0; g < 50; g++ {
		wg.Add(1)
		//3, start goroutine
		go func(gID int) {
			defer wg.Done()
			// 4, each goroutine, run a loop of 1,000 iterations
			for i := 0; i < 1000; i++ {
				mu.Lock()
				// 5,
				m[gID*1000 + i] = i
				mu.Unlock()
			}
		}(g)
	}

	wg.Wait()

	duration := time.Since(start)
	fmt.Println("Time taken:", duration)

	fmt.Println("Map length: ", len(m))
}