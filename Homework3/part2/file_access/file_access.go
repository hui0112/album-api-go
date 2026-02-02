package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

func main() {
	// The data we will write in every iteration
	const iterations = 100000
	data := []byte("test line\n")

	// Experiment 1: Unbuffered Write， every single write operation triggers a System Call
	// to the Operating System to write data to the disk.
	// This creates significant overhead.

	fmt.Println("Starting Unbuffered Write...")
	start := time.Now()

	// 1. Create/Open the file
	f1, err := os.Create("test_unbuffered.txt")
	if err != nil {
		panic(err)
	}

	// 2. Loop and write directly to the file object
	for i := 0; i < iterations; i++ {
		// f1.Write writes directly to the file (System Call)
		_, err := f1.Write(data)
		if err != nil {
			panic(err)
		}
	}

	// 3. Close the file
	f1.Close()

	// 4. Calculate and print duration
	durationUnbuffered := time.Since(start)
	fmt.Printf("Unbuffered time: %v\n\n", durationUnbuffered)

	// Experiment 2: Buffered Write， we use a "buffer" (a chunk of memory).
	// The program writes to the memory buffer first (very fast).
	// The system only writes to the disk when the buffer is full.
	// This drastically reduces the number of System Calls.

	fmt.Println("Starting Buffered Write...")
	start = time.Now()

	// 1. Create/Open the file
	f2, err := os.Create("test_buffered.txt")
	if err != nil {
		panic(err)
	}
	// Ensure file is closed at the end of the function
	defer f2.Close()

	// 2. Wrap the file writer in a bufio.Writer
	// This creates an intermediary memory buffer
	w := bufio.NewWriter(f2)

	// 3. Loop and write to the buffer
	for i := 0; i < iterations; i++ {
		// This writes to memory, not disk (very cheap operation)
		_, err := w.Write(data)
		if err != nil {
			panic(err)
		}
	}

	// 4. IMPORTANT: Flush the buffer
	// Since we wrote to memory, some data might still be sitting in RAM.
	// Flush() forces the remaining data to be written to the disk.
	err = w.Flush()
	if err != nil {
		panic(err)
	}

	// 5. Calculate and print duration
	durationBuffered := time.Since(start)
	fmt.Printf("Buffered time:   %v\n", durationBuffered)
}