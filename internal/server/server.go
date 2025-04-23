package lsp

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
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
	root        string
	handler     *protocol.Handler
	cache       cache.Cache
	parsers     map[string]*parser.Parser
	docs        map[string][]byte
	regCompiled *regexp.Regexp
	config      Config
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
		WorkspaceSymbol:         ls.workspaceSymbol,
		Shutdown:                ls.shutdown,
	}

	return server.NewServer(ls.handler, "zeta", false), nil
}

func (s *Server) URItoPath(noteuri protocol.URI) (string, error) {
	uri, err := url.Parse(noteuri)
	if err != nil {
		return "", fmt.Errorf("failed to parse uri: %w", err)
	}

	root, err := url.Parse(s.root)
	if err != nil {
		return "", fmt.Errorf("failed to parse root uri: %w", err)
	}

	if uri.Scheme != root.Scheme || uri.Host != root.Host {
		return "", fmt.Errorf("uri and root uri do not share the same scheme or host")
	}

	rel := strings.TrimPrefix(uri.Path, root.Path)
	rel = strings.TrimLeft(rel, "/") // Remove leading slash if any
	return rel, nil
}

func (s Server) PathToURI(relpath string) (string, error) {
	root, err := url.Parse(s.root)
	if err != nil {
		return "", fmt.Errorf("failed to parse root uri: %w", err)
	}

	// Join the root path and the relative path
	root.Path = path.Join(root.Path, relpath)

	// Rebuild the full URI
	return root.String(), nil
}
