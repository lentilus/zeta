package server

import (
	"fmt"
	"log"
	"time"
	"zeta/internal/cache"
	"zeta/internal/resolver"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentDidOpen(
	context *glsp.Context,
	params *protocol.DidOpenTextDocumentParams,
) error {
	note, err := resolver.Resolve(params.TextDocument.URI)
	if err != nil {
		// Not a note we manage (e.g. unsupported extension); ignore quietly.
		return nil
	}
	if _, err := s.manager.EnsureParser(note.URI); err != nil {
		return err
	}
	s.manager.UpdateDocument(note.URI, []byte(params.TextDocument.Text))
	links, meta, err := s.manager.GetLinksAndMeta(note)
	if err != nil {
		return err
	}
	if err := s.cache.EditNote(note.RelativePath, links, meta); err != nil {
		return err
	}
	// Publish against the exact URI the client sent so it can associate the
	// diagnostics with its buffer (clients match by literal URI string).
	publishDiagnostics(context, params.TextDocument.URI, s.linkDiagnostics(links))
	return nil
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	note, err := resolver.Resolve(params.TextDocument.TextDocumentIdentifier.URI)
	if err != nil {
		return nil
	}
	if _, err := s.manager.EnsureParser(note.URI); err != nil {
		return err
	}
	// Per the LSP spec, a content change with a range is an incremental edit,
	// while one without a range (delivered as TextDocumentContentChangeEventWhole)
	// replaces the entire document. Both forms are valid even under incremental
	// sync, so we must handle each.
	for _, raw := range params.ContentChanges {
		switch change := raw.(type) {
		case protocol.TextDocumentContentChangeEvent:
			if err := s.manager.ApplyIncrementalEdit(note.URI, change); err != nil {
				return fmt.Errorf("unexpected error during edit: %v", err)
			}
		case protocol.TextDocumentContentChangeEventWhole:
			if err := s.manager.ReplaceDocument(note.URI, []byte(change.Text)); err != nil {
				return fmt.Errorf("unexpected error during full update: %v", err)
			}
		default:
			return fmt.Errorf("unexpected change event type %T", raw)
		}
	}
	links, meta, err := s.manager.GetLinksAndMeta(note)
	if err != nil {
		return err
	}
	if err := s.cache.EditNote(note.RelativePath, links, meta); err != nil {
		return err
	}
	publishDiagnostics(context, params.TextDocument.URI, s.linkDiagnostics(links))
	return nil
}

func (s *Server) textDocumentDidSave(
	context *glsp.Context,
	params *protocol.DidSaveTextDocumentParams,
) error {
	note, err := resolver.Resolve(params.TextDocument.URI)
	if err != nil {
		return nil
	}
	if _, err := s.manager.EnsureParser(note.URI); err != nil {
		return err
	}
	// `text` is only present when the client honors `save.includeText`. Update
	// the stored document from it when available; otherwise keep the version
	// already maintained via didChange events.
	if params.Text != nil {
		s.manager.UpdateDocument(note.URI, []byte(*params.Text))
	}
	links, meta, err := s.manager.GetLinksAndMeta(note.URI, s.config.Query)
	if err != nil {
		return err
	}
	if err := s.cache.SaveNote(note.RelativePath, links, meta, time.Now()); err != nil {
		return err
	}
	publishDiagnostics(context, params.TextDocument.URI, s.linkDiagnostics(links))
	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	note, err := resolver.Resolve(params.TextDocument.URI)
	if err != nil {
		return nil
	}
	if err := s.cache.DiscardNote(note.RelativePath); err != nil {
		return err
	}
	s.manager.Release(note.URI)
	return nil
}

func publishDiagnostics(
	context *glsp.Context,
	uri string,
	diagnostics []protocol.Diagnostic,
) {
	context.Notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

func (s *Server) linkDiagnostics(links []cache.Link) []protocol.Diagnostic {
	diagnostics := []protocol.Diagnostic{} // empty, not nil
	// Create one diagnostic per range entry for each link
	info := protocol.DiagnosticSeverityInformation
	warn := protocol.DiagnosticSeverityWarning
	for _, l := range links {
		var severity protocol.DiagnosticSeverity
		if s.cache.NoteExists(l.Target) {
			severity = info
		} else {
			severity = warn
		}
		for _, r := range l.Ranges {
			t := string(l.Target)
			m, _ := s.cache.GetMetaData(t)
			d := protocol.Diagnostic{
				Range:    r,
				Severity: &severity,
				Message:  "> " + m["DISPLAY_TITLE"],
			}
			diagnostics = append(diagnostics, d)
		}
	}
	return diagnostics
}
