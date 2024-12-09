package lsp

import (
	"aftermath/internal/parser"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func showReferenceDiagnostics(context *glsp.Context, uri string, references []parser.Reference) {
	// Create a slice to hold the diagnostics
	var diagnostics []protocol.Diagnostic
	severity := protocol.DiagnosticSeverityInformation

	// Convert each reference into a protocol.Diagnostic
	for _, ref := range references {
		diagnostic := protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(ref.Range.StartPoint.Row),
					Character: uint32(ref.Range.StartPoint.Column),
				},
				End: protocol.Position{
					Line:      uint32(ref.Range.EndPoint.Row),
					Character: uint32(ref.Range.EndPoint.Column),
				},
			},
			Severity: &severity,         // Set the severity as Information
			Message:  "REF " + ref.Text, // Custom message based on the reference's text
		}
		diagnostics = append(diagnostics, diagnostic)
	}

	// Create the parameters for publishDiagnostics
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	// Send diagnostics using the context's Notify method
	context.Notify("textDocument/publishDiagnostics", params)
}
