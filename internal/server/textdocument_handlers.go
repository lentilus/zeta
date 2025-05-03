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
	links, err := s.manager.GetLinks(note.URI, s.config.Query)
	if err != nil {
		return err
	}
	if err := s.cache.EditNote(note.RelativePath, links); err != nil {
		return err
	}
	publishDiagnostics(context, note.URI, linkDiagnostics(links))
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
			return fmt.Errorf("unexpected error during edit:", err)
		}
	}
	links, err := s.manager.GetLinks(note.URI, s.config.Query)
	if err != nil {
		return err
	}
	if err := s.cache.EditNote(note.RelativePath, links); err != nil {
		return err
	}
	publishDiagnostics(context, note.URI, linkDiagnostics(links))
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
	links, err := s.manager.GetLinks(note.URI, s.config.Query)
	if err != nil {
		return err
	}
	if err := s.cache.SaveNote(note.RelativePath, links, time.Now()); err != nil {
		return err
	}
	publishDiagnostics(context, note.URI, linkDiagnostics(links))
	return nil
}

func (s *Server) textDocumentDidClose(
	context *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	note, _ := resolver.Resolve(params.TextDocument.URI)
	if err := s.cache.DiscardNote(note.URI); err != nil {
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
	if len(diagnostics) == 0 {
		return
	}
	context.Notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

func linkDiagnostics(links []cache.Link) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic

	// Create one diagnostic per range entry for each link
	severity := protocol.DiagnosticSeverityInformation
	for _, l := range links {
		for _, r := range l.Ranges {
			d := protocol.Diagnostic{
				Range:    r,
				Severity: &severity,
				Message:  "> " + string(l.Target),
			}
			diagnostics = append(diagnostics, d)
		}
	}

	return diagnostics
}
