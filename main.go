package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"zeta/internal/server"
)

// Version will be set during the build process using ldflags
var Version = "(dev) v0.0.0"

func main() {
	versionFlag := flag.Bool("version", false, "Print the version of the program")
	logfileFlag := flag.String("logfile", "", "Path to log file")
	dumpConfig := flag.String("dump", "", "Dump note metadata as json (path to config file)")
	flag.Parse()

	// Version
	if *versionFlag {
		fmt.Printf("zeta version %s\n", Version)
		return
	}

	// Dump command
	if *dumpConfig != "" {
		if err := runDump(*dumpConfig); err != nil {
			log.Fatalf("dump failed: %v", err)
		}
		return
	}

	// LSP server
	// 4 cores
	runtime.GOMAXPROCS(4)

	// Logging
	if *logfileFlag != "" {
		logFile, err := os.OpenFile(*logfileFlag, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
		log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
		log.Println("Starting zeta LSP server...")
	} else {
		// discard logs by default
		log.SetOutput(os.Stdout)
	}

	serverInstance, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := serverInstance.RunStdio(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
