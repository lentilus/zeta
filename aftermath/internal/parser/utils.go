package parser

import (
	"log"
	"path"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// processReferenceTarget extracts and processes the target from a reference based on the configuration
func processReferenceTarget(config Config, content string) string {
	// Compile the regex from the config
	targetRegex := regexp.MustCompile(config.TargetRegex)

	// Extract target using regex
	matches := targetRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return content // Return original content if no match
	}

	// Get the captured group and process it
	target := matches[1]

	// Replace colons with the specified path separator
	target = strings.ReplaceAll(target, config.PathSeparator, "/")

	// Check if the extension is the canonical extension
	extension := path.Ext(target)
	if extension == "" {
		// Append the canonical extension
		return target + config.CanonicalExtension
	}

	if config.CanonicalExtension != "" && extension == config.CanonicalExtension {
		// Handle ambiguity error for canonical extension, returning an empty string
		log.Println("Error, found canonical extension in reference. Illegal.")
		return ""
	}

	return target
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
