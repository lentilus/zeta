// internal/lsp/lsp.go
package lsp

import (
	"aftermath/internal/cache/memory"
	"github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type Server struct {
	root       string
	docManager memory.DocumentManager
	handler    *protocol.Handler
}

func NewServer() (*server.Server, error) {
	ls := &Server{}

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
