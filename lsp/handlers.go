package lsp

import (
	"encoding/json"
	"fmt"
	"log"
	"zeta/internal/cache"
	"zeta/internal/parser"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	// Load config
	var config Config
	configJson, err := json.Marshal(params.InitializationOptions)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(configJson, &config)
	if err != nil {
		return nil, err
	}

	log.Printf("Config: %v", config)
	s.config = config
	if params.RootURI == nil {
		return nil, fmt.Errorf("No root uri.")
	}
	s.root = *params.RootURI
	log.Printf("Root: %s", s.root)

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
	uri := params.TextDocument.URI
	log.Printf("DidOpen: %s\n", uri)

	p, err := parser.NewParser([]byte(params.TextDocument.Text))
	if err != nil {
		panic(err)
	}
	s.parsers[uri] = p

	return nil
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	uri := params.TextDocument.URI
	log.Printf("TextDocumentDidChange: %s", uri)
	p, ok := s.parsers[uri]
	if !ok {
		panic("no parser for document")
	}

	changes := params.ContentChanges
	for _, change := range changes {
		c, ok := change.(protocol.TextDocumentContentChangeEvent)
		if !ok {
			panic("text document change type not supported")
		}
		err := p.Update(c.Range.Start, c.Range.End, c.Text)
		if err != nil {
			return err
		}
	}

	err := p.Parse()
	if err != nil {
		return err
	}

	matches, err := p.Query([]byte(s.config.Query))
	if err != nil {
		return err
	}
	var links []cache.Link
	// TODO: make this robust!!!
	note := uri[len(s.root)+1:]

	log.Printf("Upserting note: %s", note)
	for _, m := range matches {
		target, err := parser.Resolve(note, m.Content)
		log.Printf("Target: %s", target)
		if err != nil {
			log.Println(err)
			continue
		}
		links = append(
			links,
			cache.Link{
				Row: uint(m.Row),
				Col: uint(m.Col),
				Ref: m.Content,
				Src: cache.Path(note),
				Tgt: cache.Path(target),
			},
		)
	}
	err = s.cache.Upsert(cache.Path(note), links)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Print("Links:")
	log.Println(s.cache.ForwardLinks(cache.Path(note)))

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
	log.Printf("textDocumentDefinition %s", uri)
	return nil, nil
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	uri := params.TextDocument.URI
	log.Printf("textDocumentReferences: %s", uri)

	return nil, nil
}
