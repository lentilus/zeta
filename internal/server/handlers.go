package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"time"
	"zeta/internal/cache"
	"zeta/internal/parser"
	"zeta/internal/resolver"
	"zeta/internal/scanner"
	"zeta/internal/sitteradapter"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	var config Config

	// Config
	configJson, err := json.Marshal(params.InitializationOptions)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(configJson, &config); err != nil {
		return nil, err
	}
	s.config = config
	log.Printf("Config: %v", config)

	// Root
	rootUri, _ := url.Parse(*params.RootURI)
	resolver.Configure(rootUri.Path, config.SelectRegex)

	// Cache File
	stateBaseDir, _ := getXDGStateHome("zeta")
	hash := sha256.New()
	hash.Write([]byte(configJson))
	configHash := hex.EncodeToString(hash.Sum(nil))
	cacheDir := path.Join(stateBaseDir, url.PathEscape(rootUri.Path), configHash)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}
	cacheFile := path.Join(cacheDir, "cache.json")

	// Restore from cache.
	dump, err := os.ReadFile(cacheFile)
	if err != nil {
		s.cache = cache.NewHybrid()
	} else {
		s.cache, err = cache.Restore(dump)
		if err != nil {
			panic(err)
		}
	}

	// Parsers
	parsers := parser.NewParserPool(10)
	s.parserPool = parsers

	// Note directory scanning + cache validation.
	seenNotes := map[cache.Path]struct{}{}
	reuseCounter := 0
	skip := func(absolutepath string, info fs.FileInfo) bool {
		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			log.Println(err)
			return true
		}
		lastSeen, _ := s.cache.Timestamp(note.CachePath)
		return lastSeen.After(info.ModTime())
	}
	now := time.Now()
	callback := func(absolutepath string, document []byte) {
		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			log.Printf("Error resolving %v", err)
		}
		seenNotes[note.CachePath] = struct{}{}
		nodes, _ := parsers.ParseAndQuery(document, []byte(s.config.Query))
		links := resolver.ExtractLinks(note, nodes, document)
		err = s.cache.Upsert(note.CachePath, links, now)
		if err != nil {
			log.Println(err)
		}
	}

	go func() {
		scanner.Scan(rootUri.Path, skip, callback)
		notes, _ := s.cache.Paths()
		for _, note := range notes {
			if _, ok := seenNotes[note]; !ok {
				// Delete handles missing Targets correclty and won't remove them
				// s.cache.Delete(note)
			}
		}
		log.Printf("Reused %d notes from cache dump.", reuseCounter)
	}()

	// Start cache dump routine.
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			log.Printf("Dumping cache to %s", cacheFile)
			dump, _ := s.cache.Dump()
			err := os.WriteFile(cacheFile, dump, 0644)
			if err != nil {
				log.Printf("Error during cache dump: %v", err)
			}
		}
	}()

	syncKind := protocol.TextDocumentSyncKindIncremental

	capabilities := s.handler.CreateServerCapabilities()
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose: &protocol.True,
		Change:    &syncKind,
		Save:      &protocol.SaveOptions{IncludeText: &protocol.True},
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
	note, _ := resolver.Resolve(params.TextDocument.URI)
	return s.process(context, note, params.TextDocument.Text)
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	uri := params.TextDocument.URI
	note, _ := resolver.Resolve(uri)

	p, ok := s.parsers[uri]
	if !ok {
		return fmt.Errorf("no parser for document %s", uri)
	}

	doc, ok := s.docs[uri]
	if !ok {
		return fmt.Errorf("no document loaded for %s", uri)
	}

	for _, raw := range params.ContentChanges {
		change, ok := raw.(protocol.TextDocumentContentChangeEvent)
		if !ok {
			return fmt.Errorf("unexpected change event type %T", raw)
		}

		// Update the Tree-sitter parser with the text edit
		tsEdit := sitteradapter.CreateTSEditAdapter(change, string(doc))
		p.Update(tsEdit)

		// Apply the same edit to our in-memory document bytes
		doc = []byte(sitteradapter.ApplyTextEdit(change, string(doc)))
	}

	// Store the updated document back in the server state.
	s.docs[uri] = doc

	return s.process(context, note, string(doc))
}

func (s *Server) textDocumentDidSave(
	context *glsp.Context,
	params *protocol.DidSaveTextDocumentParams,
) error {
	note, _ := resolver.Resolve(params.TextDocument.URI)
	document := []byte(*params.Text)

	nodes, _ := s.parserPool.ParseAndQuery(document, []byte(s.config.Query))
	links := resolver.ExtractLinks(note, nodes, document)
	s.cache.Upsert(note.CachePath, links, time.Now())

	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	uri := params.TextDocument.URI
	note, _ := resolver.Resolve(params.TextDocument.URI)

	s.cache.DeleteTmp(note.CachePath)

	// Free ressources
	delete(s.parsers, uri)
	delete(s.docs, uri)

	return nil
}

func (s *Server) shutdown(context *glsp.Context) error {
	return s.parserPool.Close()
}

func (s *Server) textDocumentDefinition(
	context *glsp.Context,
	params *protocol.DefinitionParams,
) (any, error) {
	note, _ := resolver.Resolve(params.TextDocument.URI)

	refs, err := s.cache.ForwardLinks(note.CachePath)
	if err != nil {
		return nil, err
	}
	doc, ok := s.docs[note.URI]
	if !ok {
		panic("Document not loaded")
	}

	for _, r := range refs {
		indexFrom, indexTo := r.Range.IndexesIn(string(doc))
		index := params.TextDocumentPositionParams.Position.IndexIn(string(doc))

		if index >= indexFrom && index <= indexTo {
			target, _ := resolver.Resolve(r.Tgt)
			return protocol.Location{URI: target.URI}, nil
		}
	}

	return nil, nil
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	note, _ := resolver.Resolve(params.TextDocument.URI)

	refs, err := s.cache.BackLinks(note.CachePath)
	if err != nil {
		return nil, err
	}

	var locations []protocol.Location
	for _, r := range refs {
		source, _ := resolver.Resolve(r.Src)
		locations = append(locations, protocol.Location{URI: source.URI, Range: r.Range})
	}

	return locations, nil
}

func (s *Server) workspaceSymbol(
	context *glsp.Context,
	params *protocol.WorkspaceSymbolParams,
) ([]protocol.SymbolInformation, error) {
	max_results := 128
	query := params.Query

	notes, _ := s.cache.Paths()
	counter := 0

	var symbols []protocol.SymbolInformation

	for _, note := range notes {
		path := string(note)
		if isSubsequence(query, path) {
			resolved, _ := resolver.Resolve(note)
			symbols = append(symbols, protocol.SymbolInformation{
				Name:     path,
				Kind:     protocol.SymbolKindFile,
				Location: protocol.Location{URI: resolved.URI},
			})
			counter += 1
			if counter == max_results {
				break
			}
		}
	}
	return symbols, nil
}

func (s *Server) workspaceExecuteCommand(
	context *glsp.Context,
	params *protocol.ExecuteCommandParams,
) (any, error) {
	if params.Command == "graph" {
		return nil, s.graph(context)
	}
	return nil, nil
}
