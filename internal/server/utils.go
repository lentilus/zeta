package server

import (
	"fmt"
	"os"
	"path/filepath"
	"zeta/internal/cache"
	"zeta/internal/parser"
	"zeta/internal/resolver"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

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

func linkDiagnostics(links []cache.Link) []protocol.Diagnostic {
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

// process parses, queries, caches, and publishes diagnostics.
func (s *Server) process(
	ctx *glsp.Context,
	note resolver.Note,
	document string,
) error {
	// ensure parser and doc state
	p, ok := s.parsers[note.URI]
	if !ok || p == nil {
		var err error
		p, err = parser.NewParser()
		if err != nil {
			return err
		}
		s.parsers[note.URI] = p
	}
	doc := []byte(document)
	s.docs[note.URI] = doc

	// parse
	if err := p.Parse(doc); err != nil {
		return err
	}

	// query
	nodes, err := p.Query([]byte(s.config.Query), doc)
	if err != nil {
		return err
	}

	links := resolver.ExtractLinks(note, nodes, doc)
	if err := s.cache.UpsertTmp(note.CachePath, links); err != nil {
		return err
	}

	// 6) diagnostics
	diagnostics := linkDiagnostics(links)
	if len(diagnostics) > 0 {
		ctx.Notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
			URI:         note.URI,
			Diagnostics: diagnostics,
		})
	}

	return nil
}
