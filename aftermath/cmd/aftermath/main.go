package main

import (
	"aftermath/internal/lsp"

	"github.com/tliron/commonlog"

	// Must include a backend implementation
	// See CommonLog for other options: https://github.com/tliron/commonlog
	_ "github.com/tliron/commonlog/simple"
)

func main() {
	// This increases logging verbosity (optional)
	commonlog.Configure(1, nil)

	// Initialize the protocol handler with methods tied to the LanguageServer
	server := lsp.NewServer()

	server.RunTCP("127.0.0.1:1234")
}
