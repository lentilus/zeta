package main

import (
	"aftermath/internal/lsp"
	"io"
	"log"
	"os"

	"github.com/tliron/commonlog"

	// Must include a backend implementation
	// See CommonLog for other options: https://github.com/tliron/commonlog
	_ "github.com/tliron/commonlog/simple"
)

func main() {
	// The logger for tlirons server
	commonlog.Configure(1, nil)

	// Open a log file for writing logs
	logFile, err := os.OpenFile("/tmp/amlogs", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Set up MultiWriter to log to both os.Stderr and the file
	multiWriter := io.MultiWriter(os.Stderr, logFile)
	log.SetOutput(multiWriter)

	// Initialize the protocol handler with methods tied to the LanguageServer
	server, _ := lsp.NewServer("/path/to/root")

	// Log a message before running server for debugging purposes
	log.Println("Starting server...")

	panic(server.RunStdio())
}
