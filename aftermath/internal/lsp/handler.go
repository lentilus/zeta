package lsp

import (
	"aftermath/internal/parser"
	con "context"
	"fmt"
	"log"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (ls *LanguageServer) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	capabilities := ls.handler.CreateServerCapabilities()
	syncKind := protocol.TextDocumentSyncKindIncremental
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose: &protocol.True, // Notify on open/close of documents
		Change:    &syncKind,      // Sync full document on change
	}

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &version,
		},
	}, nil
}

func (ls *LanguageServer) initialized(
	context *glsp.Context,
	params *protocol.InitializedParams,
) error {
	log.Println("Server initialized")
	return nil
}

func (ls *LanguageServer) shutdown(context *glsp.Context) error {
	log.Println("Server shutting down")
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func (ls *LanguageServer) setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	log.Printf("Trace set to: %s", params.Value)
	return nil
}

func (ls *LanguageServer) executeCommand(
	context *glsp.Context,
	params *protocol.ExecuteCommandParams,
) (interface{}, error) {
	log.Println("Hello World")
	return nil, nil
}

func reportDiagnostics(context *glsp.Context, uri string, diagnostics []protocol.Diagnostic) {
	if diagnostics == nil {
		diagnostics = []protocol.Diagnostic{}
	}
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}
	// Use the Notify function from the context to send the diagnostics
	context.Notify("textDocument/publishDiagnostics", params)
}

func (ls *LanguageServer) textDocumentDidOpen(
	context *glsp.Context,
	params *protocol.DidOpenTextDocumentParams,
) error {
	fmt.Println("Opened document")

	// Initialize parser with the full document content
	var err error
	ls.parser, err = parser.NewIncrementalParser([]byte(params.TextDocument.Text))
	if err != nil {
		return fmt.Errorf("failed to initialize parser: %w", err)
	}

	// Get references and convert them into diagnostics
	references := ls.parser.GetReferences()
	diagnostics := convertReferencesToDiagnostics(references)

	// Publish diagnostics using context's Notify function
	reportDiagnostics(context, params.TextDocument.URI, diagnostics)

	fmt.Printf("Initial References: %s", ls.parser.GetReferenceTexts())

	return nil
}

func (ls *LanguageServer) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	for _, change := range params.ContentChanges {
		switch contentChange := change.(type) {
		case protocol.TextDocumentContentChangeEventWhole:
			fmt.Println("Full Content Change")
			// Initialize or reinitialize parser with full document content
			if ls.parser != nil {
				ls.parser.Close()
			}

			var err error
			ls.parser, err = parser.NewIncrementalParser([]byte(contentChange.Text))
			if err != nil {
				return fmt.Errorf("failed to initialize parser: %w", err)
			}

		case protocol.TextDocumentContentChangeEvent:
			fmt.Println("Partial Document Change")
			// Handle incremental changes
			if ls.parser == nil {
				return fmt.Errorf("parser not initialized")
			}

			if contentChange.Range != nil {
				change := parser.DocumentChange{
					StartPos: ls.parser.CalculateOffset(parser.Position{
						Line:      uint32(contentChange.Range.Start.Line),
						Character: uint32(contentChange.Range.Start.Character),
					}),
					EndPos: ls.parser.CalculateOffset(parser.Position{
						Line:      uint32(contentChange.Range.End.Line),
						Character: uint32(contentChange.Range.End.Character),
					}),
					NewText:   []byte(contentChange.Text),
					IsPartial: true,
				}

				if err := ls.parser.ApplyChanges(con.Background(), []parser.DocumentChange{change}); err != nil {
					return fmt.Errorf("failed to apply changes: %w", err)
				}
			}
		}
	}

	// Get updated references and convert them into diagnostics
	references := ls.parser.GetReferences()
	diagnostics := convertReferencesToDiagnostics(references)

	// Publish updated diagnostics using context's Notify function
	reportDiagnostics(context, params.TextDocument.URI, diagnostics)

	fmt.Printf("References: %s", ls.parser.GetReferenceTexts())

	return nil
}

func convertReferencesToDiagnostics(references []parser.Reference) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic

	serverity := protocol.DiagnosticSeverityInformation
	for _, ref := range references {
		diagnostic := protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(ref.Range.StartPoint.Row), Character: uint32(ref.Range.StartPoint.Column)},
				End:   protocol.Position{Line: uint32(ref.Range.EndPoint.Row), Character: uint32(ref.Range.EndPoint.Column)},
			},
			Severity: &serverity,                     // You can set this to Warning, Error, etc.
			Message:  "Reference found: " + ref.Text, // Custom message based on your needs
		}
		diagnostics = append(diagnostics, diagnostic)
	}

	return diagnostics
}
