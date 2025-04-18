package sitteradapter

import (
	"strings"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
	lsp "github.com/tliron/glsp/protocol_3_16"
)

// TSEditAdapter converts an LSP TextDocumentContentChangeEvent into a tree-sitter EditInput.
func CreateTSEditAdapter(
	lspEdit lsp.TextDocumentContentChangeEvent,
	document string, // Full document content (UTF-8 encoded)
) sitter.EditInput {
	newText := lspEdit.Text

	var startByte, oldEndByte int
	var startPoint, oldEndPoint sitter.Point

	startByte, startPoint = positionToOffset(document, lspEdit.Range.Start)
	endByte, endPoint := positionToOffset(document, lspEdit.Range.End)
	oldEndByte = endByte
	oldEndPoint = endPoint

	newEndByte := startByte + len([]byte(newText))
	newEndPoint := computeNewEndPoint(startPoint, newText)

	return sitter.EditInput{
		StartIndex:  uint32(startByte),
		OldEndIndex: uint32(oldEndByte),
		NewEndIndex: uint32(newEndByte),
		StartPoint:  startPoint,
		OldEndPoint: oldEndPoint,
		NewEndPoint: newEndPoint,
	}
}

// positionToOffset computes the byte offset and tree-sitter Point for an LSP Position.
func positionToOffset(document string, pos lsp.Position) (offset int, point sitter.Point) {
	lines := strings.Split(document, "\n")
	// Clamp line number
	if int(pos.Line) >= len(lines) {
		pos.Line = uint32(len(lines) - 1)
	}
	// Sum bytes for all lines before the target line (including newline)
	for i := uint32(0); i < pos.Line; i++ {
		offset += len(lines[i]) + 1
	}
	// Traverse runes in target line to match UTF-16 character count
	var charCount, byteCount int
	for _, r := range lines[pos.Line] {
		// Each codepoint uses 1 or 2 UTF-16 code units
		unitCount := 1
		if r > 0xFFFF {
			unitCount = 2
		}
		if uint32(charCount+unitCount) > pos.Character {
			break
		}
		charCount += unitCount
		runeLen := utf8.RuneLen(r)
		byteCount += runeLen
	}
	offset += byteCount
	point = sitter.Point{Row: pos.Line, Column: uint32(byteCount)}
	return
}

// computeNewEndPoint computes the tree-sitter Point after inserting newText at startPoint.
func computeNewEndPoint(startPoint sitter.Point, newText string) sitter.Point {
	lines := strings.Split(newText, "\n")
	last := lines[len(lines)-1]
	row := startPoint.Row + uint32(len(lines)-1)
	col := uint32(len([]byte(last)))
	return sitter.Point{Row: row, Column: col}
}

// ApplyTextEdit applies a single LSP edit to the given document,
// using the same offsets that TSEditAdapter computes.
func ApplyTextEdit(
	edit lsp.TextDocumentContentChangeEvent,
	document string,
) string {
	// Compute byte offsets
	startOffset, _ := positionToOffset(document, edit.Range.Start)
	endOffset, _ := positionToOffset(document, edit.Range.End)

	// Splice the string at byte‑indices
	// (we know these are at rune boundaries because LSP always splits
	//  at code‑unit boundaries, and our positionToOffset respects that)
	before := document[:startOffset]
	after := document[endOffset:]
	return before + edit.Text + after
}

// PointToLSPPosition converts a tree-sitter Point to an LSP Position within the given document.
func TSPointToLSPPosition(pt sitter.Point, document string) lsp.Position {
	lines := strings.Split(document, "\n")
	// Clamp row to existing lines
	if int(pt.Row) >= len(lines) {
		pt.Row = uint32(len(lines) - 1)
	}
	line := lines[pt.Row]
	// Convert line to bytes and clamp column
	lineBytes := []byte(line)
	if int(pt.Column) > len(lineBytes) {
		pt.Column = uint32(len(lineBytes))
	}
	// Prefix up to the byte column
	prefix := string(lineBytes[:pt.Column])
	// Count UTF-16 code units in prefix
	var charCount uint32
	for _, r := range prefix {
		if r > 0xFFFF {
			charCount += 2
		} else {
			charCount += 1
		}
	}
	return lsp.Position{Line: pt.Row, Character: charCount}
}
