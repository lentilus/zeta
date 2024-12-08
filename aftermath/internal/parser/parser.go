package parser

import (
	"aftermath/bindings"
	"context"
	"fmt"
	"sort"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

var refQuery = []byte(`(ref) @reference`)

// Position represents a position in the document
type Position struct {
	Line      uint32
	Character uint32
}

// Reference represents a reference with its location in the code
type Reference struct {
	Text  string
	Range sitter.Range
}

// DocumentChange represents a single change to the document content
type DocumentChange struct {
	StartPos  uint32
	EndPos    uint32
	NewText   []byte
	IsPartial bool
}

// IncrementalParser holds the parser state and parsed references
type IncrementalParser struct {
	parser     *sitter.Parser
	lang       *sitter.Language
	tree       *sitter.Tree
	query      *sitter.Query
	content    []byte
	references []Reference
	mu         sync.RWMutex
}

// NewIncrementalParser creates a new IncrementalParser instance
func NewIncrementalParser(initialContent []byte) (*IncrementalParser, error) {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)

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

	if err := ip.ParseContent(context.Background(), initialContent); err != nil {
		ip.Close()
		return nil, err
	}

	return ip, nil
}

// CalculateOffset converts a Position to a byte offset in the content
func (ip *IncrementalParser) CalculateOffset(pos Position) uint32 {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	var offset uint32
	var currentLine uint32

	for i := 0; i < len(ip.content) && currentLine < pos.Line; i++ {
		if ip.content[i] == '\n' {
			currentLine++
		}
		offset++
	}

	// Add character offset
	offset += pos.Character

	// Ensure we don't exceed content length
	if offset > uint32(len(ip.content)) {
		offset = uint32(len(ip.content))
	}

	return offset
}

// ParseContent updates the content and incrementally updates the parse tree
func (ip *IncrementalParser) ParseContent(ctx context.Context, newContent []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error before parsing: %w", err)
	}

	ip.mu.Lock()
	defer ip.mu.Unlock()

	// Initial parse or complete reparse
	if len(ip.content) == 0 || ip.tree == nil {
		tree, err := ip.parser.ParseCtx(ctx, nil, newContent)
		if err != nil {
			return err
		}
		ip.tree = tree
		ip.content = newContent
		ip.references = ip.parseReferences()
		return nil
	}

	// Create edit for whole document update
	edit := sitter.EditInput{
		StartIndex:  0,
		OldEndIndex: uint32(len(ip.content)),
		NewEndIndex: uint32(len(newContent)),
		StartPoint:  sitter.Point{Row: 0, Column: 0},
		OldEndPoint: calculatePoint(ip.content),
		NewEndPoint: calculatePoint(newContent),
	}

	ip.tree.Edit(edit)
	newTree, err := ip.parser.ParseCtx(ctx, ip.tree, newContent)
	if err != nil {
		return err
	}

	if ip.tree != newTree {
		ip.tree.Close()
	}
	ip.tree = newTree
	ip.content = newContent
	ip.references = ip.parseReferences()

	return nil
}

// ApplyChanges applies multiple changes efficiently in a single operation
func (ip *IncrementalParser) ApplyChanges(ctx context.Context, changes []DocumentChange) error {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	// Optimize for single whole document change
	if len(changes) == 1 && !changes[0].IsPartial {
		return ip.ParseContent(ctx, changes[0].NewText)
	}

	// Sort changes in reverse order to apply from end to beginning
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].StartPos > changes[j].StartPos
	})

	// Create combined new content
	var newContent []byte
	currentPos := uint32(0)

	// Pre-allocate approximate size
	totalSize := len(ip.content)
	for _, change := range changes {
		totalSize += len(change.NewText) - int(change.EndPos-change.StartPos)
	}
	newContent = make([]byte, 0, totalSize)

	// Apply changes efficiently
	for _, change := range changes {
		// Add unchanged content before change
		newContent = append(newContent, ip.content[currentPos:change.StartPos]...)
		// Add new content
		newContent = append(newContent, change.NewText...)
		currentPos = change.EndPos
	}
	// Add remaining unchanged content
	newContent = append(newContent, ip.content[currentPos:]...)

	// Create single tree-sitter edit
	edit := sitter.EditInput{
		StartIndex:  0,
		OldEndIndex: uint32(len(ip.content)),
		NewEndIndex: uint32(len(newContent)),
		StartPoint:  sitter.Point{Row: 0, Column: 0},
		OldEndPoint: calculatePoint(ip.content),
		NewEndPoint: calculatePoint(newContent),
	}

	// Apply edit and parse
	ip.tree.Edit(edit)
	newTree, err := ip.parser.ParseCtx(ctx, ip.tree, newContent)
	if err != nil {
		return err
	}

	// Update state
	if ip.tree != newTree {
		ip.tree.Close()
	}
	ip.tree = newTree
	ip.content = newContent
	ip.references = ip.parseReferences()

	fmt.Printf("Entire Content:\n%s", ip.content)

	return nil
}

// parseReferences parses references from the current tree
func (ip *IncrementalParser) parseReferences() []Reference {
	if ip.tree == nil || len(ip.content) == 0 {
		return nil
	}

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(ip.query, ip.tree.RootNode())

	var refs []Reference
	for {
		match, captureIndex, ok := cursor.NextCapture()
		if !ok {
			break
		}

		if len(match.Captures) <= int(captureIndex) {
			continue
		}

		capture := match.Captures[captureIndex]
		if capture.Node == nil {
			continue
		}

		ref := Reference{
			Text:  capture.Node.Content(ip.content),
			Range: capture.Node.Range(),
		}
		refs = append(refs, ref)
	}

	return refs
}

// GetReferences returns all references with their locations
func (ip *IncrementalParser) GetReferences() []Reference {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

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

// GetReferenceAt returns the reference at the given position
func (ip *IncrementalParser) GetReferenceAt(point sitter.Point) *Reference {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	if ip.tree == nil {
		return nil
	}

	node := ip.tree.RootNode().NamedDescendantForPointRange(point, point)
	if node == nil || node.Type() != "ref" {
		return nil
	}

	ref := Reference{
		Text:  node.Content(ip.content),
		Range: node.Range(),
	}
	return &ref
}

// calculatePoint calculates the Point for a given position in the content
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

// OneTimeParser for single-use parsing
type OneTimeParser struct {
	parser *sitter.Parser
	lang   *sitter.Language
}

// NewOneTimeParser creates a new OneTimeParser instance
func NewOneTimeParser() *OneTimeParser {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)
	return &OneTimeParser{parser: parser, lang: lang}
}

// CloseParser releases parser resources
func (p *OneTimeParser) CloseParser() {
	p.parser.Close()
}

// GetReferences returns all references from the content
func (p *OneTimeParser) GetReferences(ctx context.Context, content []byte) ([]string, error) {
	tree, err := p.parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	query, err := sitter.NewQuery(refQuery, p.lang)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, tree.RootNode())

	var results []string
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			match, captureIndex, ok := cursor.NextCapture()
			if !ok {
				return results, nil
			}

			if len(match.Captures) > int(captureIndex) {
				capture := match.Captures[captureIndex]
				if capture.Node != nil {
					results = append(results, capture.Node.Content(content))
				}
			}
		}
	}
}
