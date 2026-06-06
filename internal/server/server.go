package server

import (
	"zeta/internal/cache"
	"zeta/internal/config"
	"zeta/internal/manager"
	"zeta/internal/parser"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

type Server struct {
	handler   *protocol.Handler
	cache     cache.Cache
	manager   *manager.DocumentManager
	graphAddr string
	config    config.Config
	version   string

	// TODO: I think we can always rely on the cache and get rid of this!
	// parsers is a shared pool used for one-shot parsing of notes that are
	// not currently open (e.g. resolving a goto-definition target's title).
	parsers *parser.ParserPool

	// supportsShowDocument records whether the client advertised the
	// `window/showDocument` capability during initialize. It gates the
	// optional graph feature, which has no spec-compliant fallback.
	supportsShowDocument bool

	// supportsSnippets records whether the client supports snippet completions
	// (tab stops / placeholders). When set, link completions use a `${1:…}`
	// placeholder so the inserted display text is selected after acceptance.
	supportsSnippets bool
}

func NewServer(version string) (*server.Server, error) {
	ls := &Server{version: version}
	ls.handler = &protocol.Handler{
		Initialize:              ls.initialize,
		Initialized:             ls.initialized,
		TextDocumentDidOpen:     ls.textDocumentDidOpen,
		TextDocumentDidChange:   ls.textDocumentDidChange,
		TextDocumentDidSave:     ls.textDocumentDidSave,
		TextDocumentDidClose:    ls.textDocumentDidClose,
		TextDocumentDefinition:  ls.textDocumentDefinition,
		TextDocumentReferences:  ls.textDocumentReferences,
		TextDocumentCompletion:  ls.textDocumentCompletion,
		TextDocumentCodeAction:  ls.textDocumentCodeAction,
		WorkspaceExecuteCommand: ls.workspaceExecuteCommand,
		WorkspaceSymbol:         ls.workspaceSymbol,
		Shutdown:                ls.shutdown,
	}

	return server.NewServer(ls.handler, "zeta", false), nil
}
