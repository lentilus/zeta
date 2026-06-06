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

	// Record optional client capabilities.
	if w := params.Capabilities.Window; w != nil && w.ShowDocument != nil {
		s.supportsShowDocument = w.ShowDocument.Support
	}

	// Determine the workspace root. `rootUri` is deprecated and may be null
	// (per the LSP spec), so fall back to the first workspace folder and
	// finally to the (also deprecated) rootPath before giving up.
	rootURIString, err := workspaceRoot(params)
	if err != nil {
		return nil, err
	}
	rootUri, err := url.Parse(rootURIString)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace root URI %q: %w", rootURIString, err)
	}
	resolver.Configure(
		rootUri.Path,
		config.SelectRegex,
		config.FileExtensions,
		config.DefaultExtension,
		config.TitleTemplate,
		config.TitleSubstitutions,
	)

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
	// The LSP spec requires `ExecuteCommandOptions.commands` to enumerate the
	// commands the server handles, so clients know which are available.
	capabilities.ExecuteCommandProvider = &protocol.ExecuteCommandOptions{
		Commands: []string{"graph", "createNote"},
	}
	// Offer link-target completion; `"` triggers it inside `#link("…")`.
	capabilities.CompletionProvider = &protocol.CompletionOptions{
		TriggerCharacters: []string{"\""},
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

// workspaceRoot resolves the workspace root URI from the initialize params.
// `rootUri` is preferred but is deprecated and may be null, so we fall back to
// the first workspace folder and then to the (also deprecated) `rootPath`.
func workspaceRoot(params *protocol.InitializeParams) (string, error) {
	if params.RootURI != nil && *params.RootURI != "" {
		return string(*params.RootURI), nil
	}
	if len(params.WorkspaceFolders) > 0 && params.WorkspaceFolders[0].URI != "" {
		return string(params.WorkspaceFolders[0].URI), nil
	}
	if params.RootPath != nil && *params.RootPath != "" {
		u := url.URL{Scheme: "file", Path: *params.RootPath}
		return u.String(), nil
	}
	return "", fmt.Errorf("no workspace root provided: rootUri, workspaceFolders and rootPath are all empty")
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
