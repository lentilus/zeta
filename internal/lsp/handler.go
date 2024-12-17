package lsp

import (
	"aftermath/internal/cache/memory"
	"aftermath/internal/cache/store/sqlite"
	"aftermath/internal/parser"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	root := URIToPath(*params.RootPath)
	log.Printf("Root is %s", root)

	// Load config
	var config Config
	configJson, err := json.Marshal(params.InitializationOptions)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(configJson, &config)
	if err != nil {
		log.Printf("Config error. Unable to marshall. Got %s", configJson)
		return nil, err
	}

	log.Println(config)

	// Ensure .aftermath directory exists
	aftermathDir := filepath.Join(root, ".aftermath")
	if err := os.MkdirAll(aftermathDir, 0755); err != nil {
		log.Printf("failed to create .aftermath directory: %w", err)
		return nil, fmt.Errorf("failed to create .aftermath directory: %w", err)
	}

	// Configure Reference Parser
	parserConfig := parser.Config{
		ReferenceQuery:     config.ReferenceQuery,
		TargetRegex:        config.TargetRegex,
		PathSeparator:      config.PathSeparator,
		CanonicalExtension: config.CanonicalExtension,
	}

	// Configure SQLite store
	storeConfig := sqlite.Config{
		DBPath:       filepath.Join(aftermathDir, "store.db"),
		BibPath:      filepath.Join(aftermathDir, "bibliography.yaml"),
		RootPath:     root,
		ParserConfig: parserConfig,
	}

	// Initialize SQLite store
	store, err := sqlite.NewSQLiteStore(storeConfig)
	if err != nil {
		log.Printf("failed to create store: %w")
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Configure Document Manager
	docManagerConfig := memory.Config{
		Root:         root,
		Store:        store,
		ParserConfig: parserConfig,
	}

	// Create document manager
	s.docManager = memory.NewSQLiteDocumentManager(docManagerConfig)
	s.root = root

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
	path := URIToPath(uri)
	log.Printf("DidOpen: %s\n", path)

	doc, err := s.docManager.OpenDocument(path, params.TextDocument.Text)
	if err != nil {
		log.Printf("failed to open document: %w", err)
		return fmt.Errorf("failed to open document: %w", err)
	}

	s.showReferenceDiagnostics(context, uri, doc.GetReferences())
	return nil
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	uri := params.TextDocument.URI
	path := URIToPath(uri)

	doc, exists := s.docManager.GetDocument(path)
	if !exists {
		log.Printf("document not found: %s", path)
		return fmt.Errorf("document not found: %s", path)
	}

	// Type assert the content changes
	changes := make([]memory.Change, 0, len(params.ContentChanges))
	for _, rawChange := range params.ContentChanges {
		change, ok := rawChange.(protocol.TextDocumentContentChangeEvent)
		if !ok {
			log.Printf("only incremental changes are supported")
			return fmt.Errorf("only incremental changes are supported")
		}

		changes = append(changes, memory.Change{
			Range: memory.Range{
				Start: memory.Position{
					Line:      change.Range.Start.Line,
					Character: change.Range.Start.Character,
				},
				End: memory.Position{
					Line:      change.Range.End.Line,
					Character: change.Range.End.Character,
				},
			},
			NewText: change.Text,
		})
	}

	if err := doc.ApplyChanges(changes); err != nil {
		log.Printf("failed to apply changes: %w", err)
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	s.showReferenceDiagnostics(context, uri, doc.GetReferences())
	return nil
}

func (s *Server) textDocumentDidSave(
	context *glsp.Context,
	params *protocol.DidSaveTextDocumentParams,
) error {
	path := URIToPath(params.TextDocument.URI)
	log.Printf("DidSave: %s\n", path)

	if err := s.docManager.CommitDocument(path); err != nil {
		log.Printf("failed to commit document: %w", err)
		return fmt.Errorf("failed to commit document: %w", err)
	}
	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	path := URIToPath(params.TextDocument.URI)
	log.Printf("Closed %s", path)

	// Clear diagnostics cache for closed documents
	delete(s.diagnosticCache, params.TextDocument.URI)

	if err := s.docManager.CloseDocument(path); err != nil {
		return fmt.Errorf("failed to close document: %w", err)
	}
	return nil
}

func (s *Server) shutdown(context *glsp.Context) error {
	log.Println("Shutdown")
	return s.docManager.CloseAll()
}

func (s *Server) textDocumentDefinition(
	context *glsp.Context,
	params *protocol.DefinitionParams,
) (any, error) {
	log.Println("Called go to defintion")
	path := URIToPath(params.TextDocument.URI)
	doc, exists := s.docManager.GetDocument(path)
	if !exists {
		return nil, fmt.Errorf("document not found: %s", path)
	}

	ref, found := doc.GetReferenceAt(memory.Position{
		Line:      params.Position.Line,
		Character: params.Position.Character,
	})

	if !found {
		return nil, nil
	}

	// Convert target to URI
	targetPath := filepath.Join(s.root, ref.Target)
	log.Println("Returning from go to definition")
	return protocol.Location{
		URI: PathToURI(targetPath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		},
	}, nil
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	path := URIToPath(params.TextDocument.URI)
	log.Printf("Finding Parents of: %s", path)
	parents, err := s.docManager.GetParents(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get parents: %w", err)
	}

	locations := make([]protocol.Location, len(parents))
	for i, parent := range parents {
		locations[i] = protocol.Location{
			URI: PathToURI(parent),
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 0},
			},
		}
	}

	return locations, nil
}
