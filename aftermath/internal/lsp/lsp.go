package lsp

import (
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const lsName = "aftermath"

var version string = "0.0.1"

type LanguageServer struct {
	state   string
	handler protocol.Handler
}

// NewServer initializes and returns a new LSP server instance.
func NewServer() *server.Server {
	ls := &LanguageServer{}

	// Initialize the protocol handler
	ls.handler = protocol.Handler{
		Initialize:              ls.initialize,
		Initialized:             ls.initialized,
		Shutdown:                ls.shutdown,
		SetTrace:                ls.setTrace,
		WorkspaceExecuteCommand: ls.executeCommand,
		TextDocumentDidChange:   ls.textDocumentDidChange,
		TextDocumentDidOpen:     ls.textDocumentDidOpen,
	}

	// Create the LSP server
	return server.NewServer(&ls.handler, lsName, false)
}
