package server

import (
	"fmt"
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
	note, _ := resolver.Resolve(params.TextDocument.URI)
	if _, err := s.manager.EnsureParser(note.URI); err != nil {
		return err
	}
	s.manager.UpdateDocument(note.URI, []byte(params.TextDocument.Text))
	links, meta, err := s.manager.GetLinksAndMeta(note.URI, s.config.Query)
	if err != nil {
		return err
	}
	if err := s.cache.EditNote(note.RelativePath, links, meta); err != nil {
		return err
	}
	publishDiagnostics(context, note.URI, s.linkDiagnostics(links))
	return nil
}

func (s *Server) textDocumentDidChange(
	context *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	note, _ := resolver.Resolve(params.TextDocument.TextDocumentIdentifier.URI)
	s.manager.EnsureParser(note.URI)
	for _, raw := range params.ContentChanges {
		change, ok := raw.(protocol.TextDocumentContentChangeEvent)
		if !ok {
			return fmt.Errorf("unexpected change event type %T", raw)
		}
		if err := s.manager.ApplyIncrementalEdit(note.URI, change); err != nil {
			return fmt.Errorf("unexpected error during edit: %v", err)
		}
	}
	links, meta, err := s.manager.GetLinksAndMeta(note.URI, s.config.Query)
	if err != nil {
		return err
	}
	if err := s.cache.EditNote(note.RelativePath, links, meta); err != nil {
		return err
	}
	publishDiagnostics(context, note.URI, s.linkDiagnostics(links))
	return nil
}

func (s *Server) textDocumentDidSave(
	context *glsp.Context,
	params *protocol.DidSaveTextDocumentParams,
) error {
	note, _ := resolver.Resolve(params.TextDocument.URI)
	if _, err := s.manager.EnsureParser(note.URI); err != nil {
		return err
	}
	s.manager.UpdateDocument(note.URI, []byte(*params.Text))
	links, meta, err := s.manager.GetLinksAndMeta(note.URI, s.config.Query)
	if err != nil {
		return err
	}
	if err := s.cache.SaveNote(note.RelativePath, links, meta, time.Now()); err != nil {
		return err
	}
	publishDiagnostics(context, note.URI, s.linkDiagnostics(links))
	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	note, _ := resolver.Resolve(params.TextDocument.URI)
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
				Message:  "> " + resolver.Title(t, m),
			}
			diagnostics = append(diagnostics, d)
		}
	}
	return diagnostics
}
