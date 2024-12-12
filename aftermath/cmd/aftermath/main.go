package main

import (
	"aftermath/internal/lsp"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"
)

func main() {
	// Give it some cores
	runtime.GOMAXPROCS(8)

	// Set up logging
	filename := "/tmp/aftermath.log"

	logFile, err := os.OpenFile(
		filename,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	commonlog.Configure(1, &filename)

	// Set up multi-writer for logging
	multiWriter := io.MultiWriter(os.Stderr, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Println("Starting Aftermath LSP server...")

	// Initialize the server
	server, err := lsp.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Run the server
	if err := server.RunStdio(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
