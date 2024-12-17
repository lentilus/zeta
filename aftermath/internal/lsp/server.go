package lsp

import (
	"aftermath/internal/cache/memory"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type Server struct {
	root            string
	docManager      memory.DocumentManager
	handler         *protocol.Handler
	diagnosticCache map[string][]protocol.Diagnostic // Add this line
}

func NewServer() (*server.Server, error) {
	ls := &Server{
		diagnosticCache: make(map[string][]protocol.Diagnostic), // Initialize the cache
	}

	ls.handler = &protocol.Handler{
		Initialize:             ls.initialize,
		Initialized:            ls.initialized,
		TextDocumentDidOpen:    ls.textDocumentDidOpen,
		TextDocumentDidChange:  ls.textDocumentDidChange,
		TextDocumentDidSave:    ls.textDocumentDidSave,
		TextDocumentDidClose:   ls.textDocumentDidClose,
		TextDocumentDefinition: ls.textDocumentDefinition,
		TextDocumentReferences: ls.textDocumentReferences,
		Shutdown:               ls.shutdown,
	}

	return server.NewServer(ls.handler, "aftermath", false), nil
}
