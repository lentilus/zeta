// internal/lsp/handler.go
package lsp

import (
	"aftermath/internal/cache"
	"log"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	root := *params.RootPath
	log.Printf("Root is %s", root)

	store := cache.NewStore(root)
	s.cache = store.NewCache()
	s.root = root

	capabilities := s.handler.CreateServerCapabilities()

	syncKind := protocol.TextDocumentSyncKindIncremental
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose: &protocol.True,
		Change:    &syncKind,
		Save:      true,
	}

	return protocol.InitializeResult{
		Capabilities: capabilities,
	}, nil
}

func (s *Server) initialized(
	context *glsp.Context,
	params *protocol.InitializedParams,
) error {
	log.Println("Client initialized.")
	return nil
}

func (s *Server) textDocumentDidOpen(
	context *glsp.Context,
	params *protocol.DidOpenTextDocumentParams,
) error {
	log.Printf("DidOpen: %s\n", params.TextDocument.URI)

	refs, err := s.cache.OpenDocument(
		string(params.TextDocument.URI),
		[]byte(params.TextDocument.Text),
	)
	if err != nil {
		return err
	}

	showReferenceDiagnostics(context, params.TextDocument.URI, refs)
	return nil
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	refs, err := s.cache.UpdateDocument(params.TextDocument.URI, params.ContentChanges)
	if err != nil {
		return err
	}

	showReferenceDiagnostics(context, params.TextDocument.URI, refs)
	return nil
}

func (s *Server) textDocumentDidSave(
	context *glsp.Context,
	params *protocol.DidSaveTextDocumentParams,
) error {
	log.Println("DidSave")

	s.cache.Commit(params.TextDocument.URI)
	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	log.Printf("Closed %s", params.TextDocument.URI)
	// s.cache.CloseDocument(params.TextDocument.URI)
	return nil
}

func (s *Server) shutdown(context *glsp.Context) error {
	log.Println("Shutdown")
	return nil
}

func (s *Server) textDocumentDefinition(
	context *glsp.Context,
	params *protocol.DefinitionParams,
) (any, error) {
	position, err := s.cache.ChildAt(params.TextDocument.URI, params.Position)
	if err != nil {
		return nil, err
	}
	return position, nil
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	refs, err := s.cache.Parents(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	return refs, nil
}
