package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	// 1. initialize map
	var m sync.Map
	var wg sync.WaitGroup

	start := time.Now()

	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				// 2. write data with Store
				// automatically handling concurrent safety without lock
				// but it's risky to store anything
				m.Store(gID*1000+i, i)
			}
		}(g)
	}

	wg.Wait()
	fmt.Println("Time taken:", time.Since(start))

	// 3. calculate length
	length := 0

	// Range needs a function and ANY
	m.Range(func(key, value any) bool {
		// increment length when  a new element passes
		length++

		return true // return true to keep counting，return false to stop
	})

	fmt.Println("Map length:", length)
}