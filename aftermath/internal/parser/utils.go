package parser

import (
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var (
	// Tree-sitter query to find references
	refQuery = []byte(`(ref) @reference`)

	// Regex to extract target from reference content
	// This pattern should match your specific reference format
	targetRegex = regexp.MustCompile(`^\@(.*)$`)
)

// processReferenceTarget extracts and processes the target from a reference
func processReferenceTarget(content string) string {
	// Extract target using regex
	matches := targetRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return content // Return original content if no match
	}

	// Get the captured group and process it
	target := matches[1]

	// Replace colons with forward slashes
	target = strings.ReplaceAll(target, ":", "/")

	return target + ".typ"
}

func convertPosition(pos Position) sitter.Point {
	return sitter.Point{
		Row:    pos.Line,
		Column: pos.Character,
	}
}

func calculateEndPoint(content []byte, change Change) sitter.Point {
	return convertPosition(change.Range.Start)
}
