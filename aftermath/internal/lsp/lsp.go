// internal/lsp/server.go
package lsp

import (
	"aftermath/internal/cache/memory"
	"aftermath/internal/scheduler"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type Server struct {
	root       string
	docManager memory.DocumentManager
	handler    *protocol.Handler
	scheduler  *scheduler.Scheduler
}

func NewServer(sched *scheduler.Scheduler) (*server.Server, error) {
	ls := &Server{
		scheduler: sched,
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
