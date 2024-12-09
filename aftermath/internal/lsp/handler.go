// internal/lsp/handler.go
package lsp

import (
	"aftermath/internal/parser"
	"log"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	log.Printf("Root is %s", params.RootURI)
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

func showReferenceDiagnostics(context *glsp.Context, uri string, references []parser.Reference) {
	// Create a slice to hold the diagnostics
	var diagnostics []protocol.Diagnostic
	severity := protocol.DiagnosticSeverityInformation

	// Convert each reference into a protocol.Diagnostic
	for _, ref := range references {
		diagnostic := protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(ref.Range.StartPoint.Row),
					Character: uint32(ref.Range.StartPoint.Column),
				},
				End: protocol.Position{
					Line:      uint32(ref.Range.EndPoint.Row),
					Character: uint32(ref.Range.EndPoint.Column),
				},
			},
			Severity: &severity,         // Set the severity as Information
			Message:  "REF " + ref.Text, // Custom message based on the reference's text
		}
		diagnostics = append(diagnostics, diagnostic)
	}

	// Create the parameters for publishDiagnostics
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	// Send diagnostics using the context's Notify method
	context.Notify("textDocument/publishDiagnostics", params)
}
