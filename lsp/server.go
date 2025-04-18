package lsp

import (
	"zeta/internal/cache"
	"zeta/internal/parser"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type Config struct {
	Query       string `json:"query"        required:"true"`
	SelectRegex string `json:"select_regex" required:"true"`
}

type Server struct {
	root    string
	handler *protocol.Handler
	cache   cache.Cache
	parsers map[string]*parser.Parser
	docs    map[string][]byte
	config  Config
}

func NewServer() (*server.Server, error) {
	ls := &Server{}
	ls.cache = cache.NewHybridCache()
	ls.parsers = make(map[string]*parser.Parser)
	ls.docs = make(map[string][]byte)
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
		Shutdown:                ls.shutdown,
	}

	return server.NewServer(ls.handler, "zeta", false), nil
}
