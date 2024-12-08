package lsp

import (
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
	// log.Printf("State is %s", ls.state)
	log.Println("Hello World")
	return nil, nil
}
func (ls *LanguageServer) textDocumentDidOpen(
	context *glsp.Context,
	params *protocol.DidOpenTextDocumentParams,
) error {
	fmt.Println("Opened document")
	return nil
}

// textDocumentDidChange prints document contents to stdout on each change
func (ls *LanguageServer) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	// Iterate over all content changes
	for _, change := range params.ContentChanges {
		switch contentChange := change.(type) {
		case protocol.TextDocumentContentChangeEvent:
			// Handle the change if it contains a specific range
			if contentChange.Range != nil {
				fmt.Printf("Document '%s' changed in range [%d:%d - %d:%d]:\n%s\n",
					params.TextDocument.URI,
					contentChange.Range.Start.Line, contentChange.Range.Start.Character,
					contentChange.Range.End.Line, contentChange.Range.End.Character,
					contentChange.Text)
			}
		case protocol.TextDocumentContentChangeEventWhole:
			// Handle the whole document change
			fmt.Printf("Document '%s' changed completely:\n%s\n",
				params.TextDocument.URI, contentChange.Text)
		default:
			return fmt.Errorf("unknown content change type")
		}
	}

	return nil
}
