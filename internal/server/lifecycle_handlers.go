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
	"path/filepath"
	"time"
	"zeta/internal/cache"
	"zeta/internal/manager"
	"zeta/internal/parser"
	"zeta/internal/resolver"
	"zeta/internal/scanner"

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
		s.cache = cache.NewCache()
	} else {
		s.cache, err = cache.RestoreCache(dump)
		if err != nil {
			s.cache = cache.NewCache()
		}
	}

	// Document Manager
	s.manager = manager.NewDocumentManager()

	// Parsers
	parsers := parser.NewParserPool(10)

	// Note directory scanning + cache validation.
	seenNotes := map[cache.Path]struct{}{}
	skip := func(absolutepath string, info fs.FileInfo) bool {
		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			return true
		}
		seenNotes[note.CachePath] = struct{}{}
		lastSeen := s.cache.GetSaveTime(note.CachePath)
		if err != nil {
			return false
		}

		hasNotChanged := lastSeen.After(info.ModTime())
		if !hasNotChanged {
			log.Printf("Note %s was was changed", absolutepath)
		}
		return hasNotChanged
	}
	now := time.Now()

	callback := func(absolutepath string, document []byte) {
		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			log.Printf("Unexpected error resolving %v", err)
		}
		nodes, _ := parsers.ParseAndQuery(document, []byte(s.config.Query))
		links := resolver.ExtractLinks(note, nodes, document)
		err = s.cache.SaveNote(note.CachePath, links, now)
		if err != nil {
			log.Println(err)
		}
	}

	go func() {
		scanner.Scan(rootUri.Path, skip, callback)
		notes := s.cache.GetPaths()
		for _, note := range notes {
			if _, ok := seenNotes[note]; !ok {
				s.cache.DeleteNote(note)
			}
		}
	}()

	// Start cache dump routine.
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			log.Printf("Dumping cache to %s", cacheFile)
			dump := s.cache.Dump()
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

func (s *Server) shutdown(context *glsp.Context) error {
	return nil
}

func getXDGStateHome(appName string) (string, error) {
	xdgStateHome := os.Getenv("XDG_STATE_HOME")
	if xdgStateHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		xdgStateHome = filepath.Join(homeDir, ".local", "state")
	}

	// Final path for your app
	appStateDir := filepath.Join(xdgStateHome, appName)

	// Create it if it doesn't exist
	if err := os.MkdirAll(appStateDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}

	return appStateDir, nil
}
