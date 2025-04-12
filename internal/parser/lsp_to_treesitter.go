package parser

import (
	"strings"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
	lsp "github.com/tliron/glsp/protocol_3_16"
)

// CreateTSEditAdapter constructs a Tree-sitter edit input using positions defined by your LSP library.
// It relies on the LSP library’s Position.IndexIn method to convert LSP positions (UTF-16 based)
// to UTF-8 byte offsets, which Tree-sitter expects.
func CreateTSEditAdapter(
	startPos lsp.Position, // LSP Position from the library
	endPos lsp.Position, // LSP Position from the library
	replacement string, // Replacement text to be inserted
	fullText string, // Full document content (UTF-8 encoded)
) sitter.EditInput {
	// Use the library’s public method to convert LSP positions to byte offsets.
	startByteOffset := startPos.IndexIn(fullText)
	oldEndByteOffset := endPos.IndexIn(fullText)
	// The new text’s byte length is computed with len(), since it counts bytes.
	newTextByteLength := len(replacement)
	newEndByteOffset := startByteOffset + newTextByteLength

	// If needed, compute the new LSP position corresponding to the end of the inserted text.
	// Since we cannot change library code, write your own helper for this.
	newPos := computeNewLSPPosition(startPos, replacement)

	return sitter.EditInput{
		StartIndex:  uint32(startByteOffset),
		OldEndIndex: uint32(oldEndByteOffset),
		NewEndIndex: uint32(newEndByteOffset),
		StartPoint:  sitter.Point{Row: uint32(startPos.Line), Column: uint32(startPos.Character)},
		OldEndPoint: sitter.Point{Row: uint32(endPos.Line), Column: uint32(endPos.Character)},
		NewEndPoint: sitter.Point{Row: uint32(newPos.Line), Column: uint32(newPos.Character)},
	}
}

// computeNewLSPPosition calculates the new LSP position after inserting replacement text at startPos.
// This function is defined externally and does not modify any library code.
func computeNewLSPPosition(startPos lsp.Position, replacement string) lsp.Position {
	// Split replacement text by newline to see how many lines are inserted.
	lines := strings.Split(replacement, "\n")
	if len(lines) == 0 {
		return startPos
	}
	if len(lines) == 1 {
		// The insertion did not include a newline.
		// Increase the character (UTF-16 code unit count) by the number of runes in replacement.
		return lsp.Position{
			Line:      startPos.Line,
			Character: startPos.Character + uint32(utf8.RuneCountInString(replacement)),
		}
	}
	// If the replacement spans multiple lines:
	newLine := startPos.Line + uint32(len(lines)) - 1
	newChar := uint32(utf8.RuneCountInString(lines[len(lines)-1]))
	return lsp.Position{
		Line:      newLine,
		Character: newChar,
	}
}

func ApplyTextEdit(
	startPos lsp.Position,
	endPos lsp.Position,
	replacement string,
	fullText string,
) string {
	startByteOffset := startPos.IndexIn(fullText)
	endByteOffset := endPos.IndexIn(fullText)

	// Replace the portion of fullText between startByteOffset and endByteOffset
	return fullText[:startByteOffset] + replacement + fullText[endByteOffset:]
}
