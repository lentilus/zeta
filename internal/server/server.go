package server

import (
	"zeta/internal/cache"
	"zeta/internal/config"
	"zeta/internal/manager"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type Server struct {
	handler   *protocol.Handler
	cache     cache.Cache
	manager   *manager.DocumentManager
	graphAddr string
	config    config.Config
}

func NewServer() (*server.Server, error) {
	ls := &Server{}
	ls.handler = &protocol.Handler{
		Initialize:              ls.initialize,
		Initialized:             ls.initialized,
		TextDocumentDidOpen:     ls.textDocumentDidOpen,
		TextDocumentDidChange:   ls.textDocumentDidChange,
		TextDocumentDidSave:     ls.textDocumentDidSave,
		TextDocumentDidClose:    ls.textDocumentDidClose,
		TextDocumentDefinition:  ls.textDocumentDefinition,
		TextDocumentReferences:  ls.textDocumentReferences,
		WorkspaceExecuteCommand: ls.workspaceExecuteCommand,
		WorkspaceSymbol:         ls.workspaceSymbol,
		Shutdown:                ls.shutdown,
	}

	return server.NewServer(ls.handler, "zeta", false), nil
}
