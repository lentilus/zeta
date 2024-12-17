package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"zeta/internal/lsp"

	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"
)

// Version will be set during the build process using ldflags
var Version = "-dev-"

func main() {
	// Define and parse the --version flag
	versionFlag := flag.Bool("version", false, "Print the version of the program")
	flag.Parse()

	// Print the version if the flag is set
	if *versionFlag {
		fmt.Printf("zeta LSP server version %s\n", Version)
		return
	}

	// Give it some cores
	runtime.GOMAXPROCS(8)

	// Set up logging
	filename := "/tmp/zeta.log"

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
	log.Println("Starting zeta LSP server...")

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
