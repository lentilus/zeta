package parser

import (
	"aftermath/bindings"
	"context"
	"fmt"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

var refQuery = []byte(`(ref) @reference`)

// Reference represents a reference with its location in the code
type Reference struct {
	Text  string
	Start uint32
	End   uint32
	Line  uint32
	Col   uint32
}

// Position represents a location in the source code
type Position struct {
	Line uint32
	Col  uint32
}

// IncrementalParser holds the parser state and parsed references
type IncrementalParser struct {
	parser     *sitter.Parser
	lang       *sitter.Language
	tree       *sitter.Tree
	query      *sitter.Query
	content    []byte
	references []Reference // Cache for references with locations
	mu         sync.RWMutex
}

// NewIncrementalParser creates a new IncrementalParser instance
func NewIncrementalParser(initialContent []byte) (*IncrementalParser, error) {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)

	// Create the query once during initialization
	query, err := sitter.NewQuery(refQuery, lang)
	if err != nil {
		parser.Close()
		return nil, err
	}

	ip := &IncrementalParser{
		parser:  parser,
		lang:    lang,
		query:   query,
		content: initialContent,
	}

	// Parse initial content
	if err := ip.ParseContent(context.Background(), initialContent); err != nil {
		ip.Close()
		return nil, err
	}

	return ip, nil
}

// ParseContent updates the content and incrementally updates the parse tree
func (ip *IncrementalParser) ParseContent(ctx context.Context, newContent []byte) error {
	// First check if context is already canceled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error before parsing: %w", err)
	}

	ip.mu.Lock()
	defer ip.mu.Unlock()

	// If there's no existing content, just do a fresh parse
	if len(ip.content) == 0 {
		tree, err := ip.parser.ParseCtx(ctx, nil, newContent)
		if err != nil {
			return err
		}
		ip.tree = tree
		ip.content = newContent
		ip.references = ip.parseReferences()
		return nil
	}

	// Calculate the edit between old and new content
	edit := calculateEdit(ip.content, newContent)

	// Apply the edit to the existing tree
	if ip.tree != nil {
		ip.tree.Edit(edit)
	}

	// Create a channel to handle parser timeout/cancellation
	done := make(chan struct{})
	var parseErr error
	var newTree *sitter.Tree

	go func() {
		defer close(done)
		// Use the existing tree for incremental parsing
		newTree, parseErr = ip.parser.ParseCtx(ctx, ip.tree, newContent)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		if parseErr != nil {
			return parseErr
		}
	}

	// Close old tree if new tree is different
	if ip.tree != nil && ip.tree != newTree {
		ip.tree.Close()
	}

	ip.tree = newTree
	ip.content = newContent

	// Update references cache
	ip.references = ip.parseReferences()

	return nil
}

// calculateEdit computes the edit between old and new content
func calculateEdit(oldContent, newContent []byte) sitter.EditInput {
	// Find the start of the difference
	startByte := 0
	for startByte < len(oldContent) && startByte < len(newContent) {
		if oldContent[startByte] != newContent[startByte] {
			break
		}
		startByte++
	}

	// Find the end of the difference
	oldEndByte := len(oldContent)
	newEndByte := len(newContent)
	for oldEndByte > startByte && newEndByte > startByte {
		if oldContent[oldEndByte-1] != newContent[newEndByte-1] {
			break
		}
		oldEndByte--
		newEndByte--
	}

	// Calculate start position (row, column)
	startPoint := calculatePoint(oldContent[:startByte])

	// Calculate old end position
	oldEndPoint := calculatePoint(oldContent[:oldEndByte])

	// Calculate new end position
	newEndPoint := calculatePoint(newContent[:newEndByte])

	return sitter.EditInput{
		StartIndex:  uint32(startByte),
		OldEndIndex: uint32(oldEndByte),
		NewEndIndex: uint32(newEndByte),
		StartPoint:  startPoint,
		OldEndPoint: oldEndPoint,
		NewEndPoint: newEndPoint,
	}
}

// calculatePoint calculates the Point (row, column) for a given position in the content
func calculatePoint(content []byte) sitter.Point {
	row := uint32(0)
	column := uint32(0)

	for _, b := range content {
		if b == '\n' {
			row++
			column = 0
		} else {
			column++
		}
	}

	return sitter.Point{
		Row:    row,
		Column: column,
	}
}

// parseReferences parses references from the current tree
// This is an internal method that should be called with the lock held
func (ip *IncrementalParser) parseReferences() []Reference {
	if ip.tree == nil || len(ip.content) == 0 {
		return nil
	}

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(ip.query, ip.tree.RootNode())

	var refs []Reference
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		match = cursor.FilterPredicates(match, ip.content)
		for _, capture := range match.Captures {
			// Add safety checks
			if capture.Node == nil {
				continue
			}

			start := capture.Node.StartByte()
			end := capture.Node.EndByte()

			// Bounds checking
			if start >= uint32(len(ip.content)) || end > uint32(len(ip.content)) || start >= end {
				continue
			}

			ref := Reference{
				Text:  string(ip.content[start:end]),
				Start: start,
				End:   end,
				Line:  capture.Node.StartPoint().Row,
				Col:   capture.Node.StartPoint().Column,
			}
			refs = append(refs, ref)
		}
	}

	return refs
}

// GetReferences returns all references with their locations
func (ip *IncrementalParser) GetReferences() []Reference {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	// Return a copy to prevent modification of cache
	refs := make([]Reference, len(ip.references))
	copy(refs, ip.references)
	return refs
}

// GetReferenceTexts returns just the text of all references
func (ip *IncrementalParser) GetReferenceTexts() []string {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	texts := make([]string, len(ip.references))
	for i, ref := range ip.references {
		texts[i] = ref.Text
	}
	return texts
}

// GetReferenceAt returns the reference at the given position, or nil if there isn't one
func (ip *IncrementalParser) GetReferenceAt(pos Position) *Reference {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	// Check each reference to see if the position falls within its range
	for _, ref := range ip.references {
		// If we're on the right line
		if ref.Line == pos.Line {
			// Check if the position falls within the reference's column range
			refEndCol := ref.Col + uint32(len(ref.Text))
			if pos.Col >= ref.Col && pos.Col < refEndCol {
				// Return a copy of the reference to prevent modification of cached data
				refCopy := ref
				return &refCopy
			}
		}
	}

	return nil
}

// Close releases all resources
func (ip *IncrementalParser) Close() {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	if ip.query != nil {
		ip.query.Close()
	}
	if ip.tree != nil {
		ip.tree.Close()
	}
	if ip.parser != nil {
		ip.parser.Close()
	}
}

type OneTimeParser struct {
	parser *sitter.Parser
	lang   *sitter.Language
}

func NewOneTimeParser() *OneTimeParser {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)
	return &OneTimeParser{parser: parser, lang: lang}
}

func (parser *OneTimeParser) CloseParser() {
	parser.parser.Close()
}

// Get zettel references by a treesitter query from file content.
func (parser *OneTimeParser) GetReferences(content []byte) ([]string, error) {
	// Parse the source code
	tree := parser.parser.Parse(nil, content)
	defer tree.Close()

	// Query the tree
	query, err := sitter.NewQuery([]byte(refQuery), parser.lang)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, tree.RootNode())

	var results []string
	// Iterate over all matches
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// Apply predicates filtering
		m = cursor.FilterPredicates(m, content)
		for _, c := range m.Captures {
			if c.Node != nil {
				results = append(results, c.Node.Content(content))
			}
		}
	}

	return results, nil
}
