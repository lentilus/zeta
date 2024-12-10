package main

import (
	"aftermath/internal/lsp"
	"aftermath/internal/scheduler"
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

	logFile, err := os.OpenFile("/tmp/amlogs", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	multiWriter := io.MultiWriter(os.Stderr, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("Starting server...")

	// Initialize scheduler
	sched := scheduler.NewScheduler(16)
	sched.RunScheduler()
	defer sched.StopScheduler()

	// Initialize the protocol handler with methods tied to the LanguageServer
	server, _ := lsp.NewServer(sched)

	log.Panic(server.RunStdio())
}
