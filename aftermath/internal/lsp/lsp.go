// internal/lsp/server.go
package lsp

import (
	"aftermath/internal/cache"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type Server struct {
	cache   *cache.Cache
	handler *protocol.Handler
}

func NewServer(root string) (*server.Server, error) {
	store := cache.NewStore(root)
	cache := store.NewCache()

	ls := &Server{
		cache: cache,
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
