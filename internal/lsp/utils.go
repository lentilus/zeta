package lsp

import (
	"reflect"
	"strings"
	"zeta/internal/cache/memory"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) showReferenceDiagnostics(
	context *glsp.Context,
	uri string,
	references []memory.Reference,
) {
	diagnostics := make([]protocol.Diagnostic, len(references))
	severity := protocol.DiagnosticSeverityInformation

	for i, ref := range references {
		diagnostics[i] = protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      ref.Range.Start.Line,
					Character: ref.Range.Start.Character,
				},
				End: protocol.Position{
					Line:      ref.Range.End.Line,
					Character: ref.Range.End.Character,
				},
			},
			Severity: &severity,
			Message:  "> " + ref.Target,
		}
	}

	// Check if diagnostics have changed
	if previous, exists := s.diagnosticCache[uri]; exists {
		if reflect.DeepEqual(previous, diagnostics) {
			return // Skip publishing if diagnostics haven't changed
		}
	}

	// Update cache and publish new diagnostics
	s.diagnosticCache[uri] = diagnostics
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	context.Notify("textDocument/publishDiagnostics", params)
}

// URIToPath converts a LSP URI to a filesystem path
func URIToPath(uri string) string {
	// Remove 'file://' prefix
	path := strings.TrimPrefix(uri, "file://")
	// Convert any remaining URI encoding
	path = strings.ReplaceAll(path, "%20", " ")
	return path
}

// PathToURI converts a filesystem path to a LSP URI
func PathToURI(path string) string {
	// Replace spaces with %20
	uri := strings.ReplaceAll(path, " ", "%20")
	// Add file:// prefix
	return "file://" + uri
}
