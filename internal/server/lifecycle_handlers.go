package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
	"zeta/internal/cache"
	"zeta/internal/config"
	"zeta/internal/embed"
	"zeta/internal/manager"
	"zeta/internal/parser"
	"zeta/internal/resolver"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	config, err := config.Load(params.InitializationOptions)
	if err != nil {
		return nil, err
	}

	// TODO: document that we are overriding config root here
	if params.RootURI == nil {
		return nil, errors.New("No Root given")
	}


	rootUri, err := url.Parse(*params.RootURI)
	if err != nil {
		return nil, err
	}
	config.Root = rootUri.Path

	parser.AddFormat("typst", config.Typst)
	parser.AddFormat("markdown", config.Markdown)

	s.config = config

	log.Printf("Config: %v", s.config)

	err = resolver.Configure(config.Root, config.Extensions)
	if err != nil {
		return nil, err
	}

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
	// Cache File
	stateBaseDir, _ := getXDGStateHome("zeta")
	hash := sha256.New()
	b, err := json.Marshal(s.config)
	if err != nil {
		return err
	}
	hash.Write([]byte(b))
	configHash := hex.EncodeToString(hash.Sum(nil))
	cacheDir := path.Join(stateBaseDir, url.PathEscape(s.config.Root), configHash)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	cacheFile := path.Join(cacheDir, "cache.json")

	s.manager = manager.NewDocumentManager()
	s.embedder = embed.NewTestEmbedder()

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

	log.Println("Starting indexing")
	err = s.indexNotes(context)
	if err != nil {
	    return err
	}

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

	appStateDir := filepath.Join(xdgStateHome, appName)

	// Create it if it doesn't exist
	if err := os.MkdirAll(appStateDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}

	return appStateDir, nil
}
