package parser

import (
	"aftermath/bindings"
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
}

// NewIncrementalParser creates a new IncrementalParser instance
func NewIncrementalParser(initialContent []byte) *IncrementalParser {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)

	ip := &IncrementalParser{
		parser:  parser,
		lang:    lang,
		content: initialContent,
	}
	
	// Parse initial content
	ip.tree = parser.Parse(nil, initialContent)
	return ip
}

// Parse updates the content and incrementally updates the parse tree and references
func (ip *IncrementalParser) Parse(newContent []byte) error {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	// Perform incremental parse
	oldTree := ip.tree
	ip.tree = ip.parser.ParseWithOptions(oldTree, newContent, sitter.ParseOptions{
		Encoding: sitter.Encoding(sitter.UTF8),
	})
	ip.content = newContent

	// Update references using the new tree
	query, err := sitter.NewQuery(refQuery, ip.lang)
	if err != nil {
		return err
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
	return nil
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
