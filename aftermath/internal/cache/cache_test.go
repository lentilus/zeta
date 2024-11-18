package cache_test

import (
	"aftermath/internal/cache"
	"fmt"
	"os"
	"runtime"
	"runtime/trace"
	"testing"
)

func TestUpdateIncremental(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Open a file to store the trace profile
	f, err := os.Create("trace.out")
	if err != nil {
		fmt.Println("could not create trace file:", err)
		return
	}
	defer f.Close()

	// Start trace profiling
	if err := trace.Start(f); err != nil {
		fmt.Println("could not start trace:", err)
		return
	}
	defer trace.Stop() // Stop tracing when main exits

	// Directory to walk (replace this with the actual directory you want to test on)
	kasten := cache.NewZettelkasten(
		"/home/lentilus/typstest",
		"/home/lentilus/typstest/cache.sqlite",
	)

	// Run the main program logic
	kasten.UpdateIncremental()
}
