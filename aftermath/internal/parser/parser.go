package parser

import (
	"aftermath/bindings"
	"context"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

var refQuery = []byte(`((ref) @reference)`)

// IncrementalParser holds the parser state and parsed references
type IncrementalParser struct {
	parser     *sitter.Parser
	lang       *sitter.Language
	tree       *sitter.Tree
	content    []byte
	references []string
	mu         sync.RWMutex
	reader     *contentReader
}

// contentReader implements tree-sitter's Input.Read interface
type contentReader struct {
	content []byte
	offset  uint32
}

func (r *contentReader) Read(offset uint32) []byte {
	if offset >= uint32(len(r.content)) {
		return nil
	}
	return r.content[offset:]
}

// NewIncrementalParser creates a new IncrementalParser instance
func NewIncrementalParser(initialContent []byte) *IncrementalParser {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)

	reader := &contentReader{content: initialContent}
	ip := &IncrementalParser{
		parser:  parser,
		lang:    lang,
		content: initialContent,
		reader:  reader,
	}

	// Parse initial content
	input := sitter.Input{
		Read: func(offset uint32, position sitter.Point) []byte {
			return reader.Read(offset)
		},
	}
	ip.tree = parser.ParseInput(nil, input)
	
	// Parse initial references
	query, err := sitter.NewQuery(refQuery, ip.lang)
	if err != nil {
		return ip // Return even if query fails
	}

	cursor := sitter.NewQueryCursor()
	cursor.Exec(query, ip.tree.RootNode())

	var initialRefs []string
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		m = cursor.FilterPredicates(m, initialContent)
		for _, c := range m.Captures {
			initialRefs = append(initialRefs, c.Node.Content(initialContent))
		}
	}

	ip.references = initialRefs
	return ip
}

// Parse updates the content and incrementally updates the parse tree and references.
// Returns the updated tree-sitter Tree.
func (ip *IncrementalParser) Parse(ctx context.Context, newContent []byte) (*sitter.Tree, error) {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	// Update reader with new content
	ip.reader.content = newContent

	// Create input for tree-sitter
	input := sitter.Input{
		Read: func(offset uint32, position sitter.Point) []byte {
			return ip.reader.Read(offset)
		},
	}

	// Perform incremental parse with context
	oldTree := ip.tree
	tree, err := ip.parser.ParseInputCtx(ctx, oldTree, input)
	if err != nil {
		return nil, err
	}

	ip.tree = tree
	ip.content = newContent

	// Update references using the new tree
	query, err := sitter.NewQuery(refQuery, ip.lang)
	if err != nil {
		return nil, err
	}

	cursor := sitter.NewQueryCursor()
	cursor.Exec(query, ip.tree.RootNode())

	var newRefs []string
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		m = cursor.FilterPredicates(m, newContent)
		for _, c := range m.Captures {
			newRefs = append(newRefs, c.Node.Content(newContent))
		}
	}

	ip.references = newRefs
	return tree, nil
}

// GetReferences returns the parsed references in a thread-safe manner
func (ip *IncrementalParser) GetReferences() []string {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	// Return a copy to prevent external modifications
	refs := make([]string, len(ip.references))
	copy(refs, ip.references)
	return refs
}

// Close releases resources
func (ip *IncrementalParser) Close() {
	if ip.tree != nil {
		ip.tree.Close()
	}
	ip.parser.Close()
}

type Parser struct {
	parser *sitter.Parser
	lang   *sitter.Language
}

func NewParser() *Parser {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)
	return &Parser{parser: parser, lang: lang}
}

func (parser *Parser) CloseParser() {
	parser.parser.Close()
}

// Get zettel references by a treesitter query from file content.
func (parser *Parser) GetReferences(content []byte) ([]string, error) {

	// Parse the source code
	tree := parser.parser.Parse(nil, content)
	defer tree.Close()

	// Query the tree
	query, err := sitter.NewQuery([]byte(refQuery), parser.lang)
	if err != nil {
		return []string{}, err
	}
	cursor := sitter.NewQueryCursor()
	cursor.Exec(query, tree.RootNode())

	results := []string{}
	// Iterate over all matches
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// Apply predicates filtering
		m = cursor.FilterPredicates(m, content)
		for _, c := range m.Captures {
			results = append(results, c.Node.Content(content))
		}
	}

	return results, nil
}
