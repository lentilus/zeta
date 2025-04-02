package lsp

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {

	// Load config
	var config any
	configJson, err := json.Marshal(params.InitializationOptions)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(configJson, &config)
	if err != nil {
		return nil, err
	}

	log.Println(config)

	capabilities := s.handler.CreateServerCapabilities()

	syncKind := protocol.TextDocumentSyncKindIncremental
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose: &protocol.True,
		Change:    &syncKind,
		Save:      true,
	}

	log.Println("Returning from initialize")
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
	uri := params.TextDocument.URI
	log.Printf("DidOpen: %s\n", uri)

	return nil
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	uri := params.TextDocument.URI
	fmt.Printf("TextDocumentDidChange: %s", uri)

	return nil
}

func (s *Server) textDocumentDidSave(
	context *glsp.Context,
	params *protocol.DidSaveTextDocumentParams,
) error {
	uri := params.TextDocument.URI
	log.Printf("DidSave: %s\n", uri)

	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	uri := params.TextDocument.URI
	log.Printf("Closed %s", uri)

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
	log.Println("Called go to defintion")
	uri := params.TextDocument.URI
	fmt.Printf("textDocumentDefinition %s", uri)
	return nil, nil
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	uri := params.TextDocument.URI
	fmt.Printf("textDocumentReferences: %s", uri)

	return nil, nil
}
