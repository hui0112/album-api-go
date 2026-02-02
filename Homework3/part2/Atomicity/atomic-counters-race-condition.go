package main

import (
    "fmt"
    "sync"
    // sync/atomic
)

func main() {
    //use regular int instead
    var ops int
    var wg sync.WaitGroup

    for range 50 {
        wg.Add(1) // 1, check in
        go func() {
            defer wg.Done() // check out no matter what
            for range 1000 {
                // 2. regular addition
                ops++
            }
        }()
    }

    wg.Wait() // wait until all processes done
    fmt.Println("ops (regular int):", ops)
}