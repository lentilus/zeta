package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
	"time"
	"zeta/internal/cache"
	"zeta/internal/parser"
	"zeta/internal/scanner"
	"zeta/internal/sitteradapter"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	var config Config

	configJson, err := json.Marshal(params.InitializationOptions)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJson, &config); err != nil {
		return nil, err
	}

	log.Printf("Config: %v", config)
	s.config = config

	if params.RootURI == nil {
		return nil, fmt.Errorf("no root URI")
	}

	s.root = *params.RootURI

	stateBaseDir, _ := getXDGStateHome("zeta")
	cacheDir := path.Join(stateBaseDir, url.PathEscape(s.root))
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}

	cacheFile := path.Join(cacheDir, "cache.json")
	dump, err := os.ReadFile(cacheFile)
	if err != nil {
		s.cache = cache.NewHybrid()
	} else {
		s.cache, err = cache.Restore(dump)
		if err != nil {
			panic(err)
		}
	}

	s.regCompiled, err = regexp.Compile(s.config.SelectRegex)
	if err != nil {
		return nil, err
	}

	parsers := parser.NewParserPool(10)
	s.parserPool = parsers
	skip := func(path string, info fs.FileInfo) bool {
		lastSeen, _ := s.cache.Timestamp(cache.Path(path))
		return lastSeen.After(info.ModTime())
	}
	now := time.Now()
	callback := func(path string, document []byte) {
		nodes, _ := parsers.ParseAndQuery(document, []byte(s.config.Query))
		links, _ := extractLinks(nodes, document, path, s.regCompiled)
		err := s.cache.Upsert(cache.Path(path), links, now)
		log.Println(err)
	}

	go scanner.Scan(s.root[len("file://"):], skip, callback)

	// Start cache dump routine.
	ticker := time.NewTicker(1 * time.Minute)
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
	return s.process(context, params.TextDocument.URI, params.TextDocument.Text)
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	uri := params.TextDocument.URI

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

	return s.process(context, uri, string(doc))
}

func (s *Server) textDocumentDidSave(
	context *glsp.Context,
	params *protocol.DidSaveTextDocumentParams,
) error {
	document := []byte(*params.Text)
	uri := params.TextDocument.URI
	path, _ := s.URItoPath(uri)

	now := time.Now()

	parsers := s.parserPool
	nodes, _ := parsers.ParseAndQuery(document, []byte(s.config.Query))
	links, _ := extractLinks(nodes, document, path, s.regCompiled)
	s.cache.Upsert(cache.Path(path), links, now)

	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	uri := params.TextDocument.URI

	path, _ := s.URItoPath(uri)
	s.cache.Delete(cache.Path(path))

	// Free ressources
	delete(s.parsers, uri)
	delete(s.docs, uri)

	return nil
}

func (s *Server) shutdown(context *glsp.Context) error {
	_ = s.parserPool.Close()
	// TODO: close dangling parsers in s.parsers
	return nil
}

func (s *Server) textDocumentDefinition(
	context *glsp.Context,
	params *protocol.DefinitionParams,
) (any, error) {
	log.Println("Called go to definition")
	log.Printf("textDocumentDefinition %s", params.TextDocument.URI)

	uri := params.TextDocument.URI

	path, _ := s.URItoPath(uri)

	refs, err := s.cache.ForwardLinks(cache.Path(path))
	if err != nil {
		return nil, err
	}
	doc, ok := s.docs[uri]
	if !ok {
		panic("Document not loaded")
	}

	for _, r := range refs {
		indexFrom, indexTo := r.Range.IndexesIn(string(doc))
		index := params.TextDocumentPositionParams.Position.IndexIn(string(doc))

		if index >= indexFrom && index <= indexTo {
			locUri, _ := s.PathToURI(string(r.Tgt))
			return protocol.Location{URI: locUri}, nil
		}
	}

	return nil, nil
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	uri := params.TextDocument.URI

	path, _ := s.URItoPath(uri)

	refs, err := s.cache.BackLinks(cache.Path(path))
	if err != nil {
		return nil, err
	}

	var locations []protocol.Location
	for _, r := range refs {
		locUri, _ := s.PathToURI(string(r.Src))
		locRange := r.Range

		locations = append(locations, protocol.Location{URI: locUri, Range: locRange})
	}

	return locations, nil
}

func (s *Server) workspaceSymbol(
	context *glsp.Context,
	params *protocol.WorkspaceSymbolParams,
) ([]protocol.SymbolInformation, error) {
	max_results := 100
	query := params.Query

	notes, _ := s.cache.Paths()
	counter := 0

	var symbols []protocol.SymbolInformation

	for _, note := range notes {
		path := string(note)
		if isSubsequence(query, path) {
			uri, _ := s.PathToURI(path)
			symbols = append(symbols, protocol.SymbolInformation{
				Name:     path,
				Kind:     protocol.SymbolKindFile,
				Location: protocol.Location{URI: uri},
			})
			if counter == max_results {
				break
			}
			counter += 1
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

func extractLinks(
	nodes []*sitter.Node,
	document []byte,
	source string,
	reg *regexp.Regexp,
) ([]cache.Link, error) {
	var links []cache.Link

	for _, n := range nodes {
		content := (*n).Content(document)
		match := reg.FindSubmatch([]byte(content))[1]
		if match == nil {
			continue
		}

		target, err := parser.Resolve(source, string(match))
		if err != nil {
			continue
		}

		log.Printf("Treesitter range is: %v", (*n).Range())

		l := cache.Link{
			Range: protocol.Range{
				Start: sitteradapter.TSPointToLSPPosition((*n).StartPoint(), string(document)),
				End:   sitteradapter.TSPointToLSPPosition((*n).EndPoint(), string(document)),
			},
			Src: cache.Path(source),
			Tgt: cache.Path(target),
		}

		log.Printf("Link is: %v", l)
		links = append(links, l)
	}

	return links, nil
}

func linkDiagnostics(links []cache.Link) []protocol.Diagnostic {
	log.Println("Showing diagnostics")

	var diagnostics []protocol.Diagnostic

	for _, l := range links {
		severity := protocol.DiagnosticSeverityInformation

		d := protocol.Diagnostic{
			Range:    l.Range,
			Severity: &severity,
			Message:  "> " + string(l.Tgt),
		}

		diagnostics = append(diagnostics, d)
	}

	return diagnostics
}

func isSubsequence(pattern, text string) bool {
	// convert pattern to runes so we compare full Unicode codepoints
	pr := []rune(pattern)
	if len(pr) == 0 {
		return true // empty pattern always matches
	}

	i := 0
	for _, r := range text {
		if r == pr[i] {
			i++
			if i == len(pr) {
				return true
			}
		}
	}
	return false
}

// process parses, queries, caches, and publishes diagnostics.
func (s *Server) process(
	ctx *glsp.Context,
	uri protocol.DocumentUri,
	document string,
) error {
	// ensure parser and doc state
	p, ok := s.parsers[uri]
	if !ok || p == nil {
		var err error
		p, err = parser.NewParser()
		if err != nil {
			return err
		}
		s.parsers[uri] = p
	}
	doc := []byte(document)
	s.docs[uri] = doc

	// parse
	if err := p.Parse(doc); err != nil {
		return err
	}

	// query
	nodes, err := p.Query([]byte(s.config.Query), doc)
	if err != nil {
		return err
	}

	// build links
	source, err := s.URItoPath(uri)
	if err != nil {
		return err
	}
	links, err := extractLinks(nodes, doc, source, s.regCompiled)
	if err != nil {
		return err
	}

	// cache
	path := cache.Path(source)
	if err := s.cache.UpsertTmp(path, links); err != nil {
		return err
	}

	// 6) diagnostics
	diagnostics := linkDiagnostics(links)
	if len(diagnostics) > 0 {
		ctx.Notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnostics,
		})
	}

	return nil
}
