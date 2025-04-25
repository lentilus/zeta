package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"zeta/internal/server"

	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"
)

// Version will be set during the build process using ldflags
var Version = "(dev) v0.0.0"

func main() {
	versionFlag := flag.Bool("version", false, "Print the version of the program")
	logfileFlag := flag.String("logfile", "", "Path to log file")
	flag.Parse()

	// Version tag
	if *versionFlag {
		fmt.Printf("zeta LSP server version %s\n", Version)
		return
	}

	// 4 Cores
	runtime.GOMAXPROCS(4)

	// Logging
	if *logfileFlag != "" {
		logFile, err := os.OpenFile(*logfileFlag, os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
		log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
		log.Println("Starting zeta LSP server...")
	} else {
		log.SetOutput(io.Discard)
	}
	commonlog.Configure(2, nil) // Logger used by glsp

	// Initialize the server
	server, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Run the server
	if err := server.RunStdio(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
