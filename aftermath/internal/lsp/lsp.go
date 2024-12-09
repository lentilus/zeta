// internal/lsp/server.go
package lsp

import (
	"aftermath/internal/cache"
	"context"
	"sync"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type clientIDKey struct{}

type Server struct {
	store   *cache.Store
	clients map[context.Context]*cache.Cache
	mu      sync.RWMutex
	handler protocol.Handler
}

func NewServer(root string) (*server.Server, error) {
	store := cache.NewStore(root)

	ls := &Server{
		store:   store,
		clients: make(map[context.Context]*cache.Cache),
	}

	ls.handler = protocol.Handler{
		Initialize:          ls.initialize,
		Initialized:         ls.initialized,
		Shutdown:            ls.shutdown,
		TextDocumentDidOpen: ls.textDocumentDidOpen,
	}

	return server.NewServer(&ls.handler, "aftermath", false), nil
}

func (s *Server) getOrCreateCache(ctx context.Context) *cache.Cache {
	s.mu.Lock()
	defer s.mu.Unlock()

	if client, exists := s.clients[ctx]; exists {
		return client
	}

	client := s.store.NewCache()
	s.clients[ctx] = client
	return client
}
