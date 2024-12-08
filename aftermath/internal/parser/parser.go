package parser

import (
	"aftermath/bindings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

var refQuery = []byte(`((ref) @reference)`)

// IncrementalParser holds the parser state and parsed references
type IncrementalParser struct {
	parser     *Parser
	content    []byte
	references []string
	mu         sync.RWMutex
}

// NewIncrementalParser creates a new IncrementalParser instance
func NewIncrementalParser(content []byte) *IncrementalParser {
	return &IncrementalParser{
		parser:  NewParser(),
		content: content,
	}
}

// Parse parses the content and stores references in a thread-safe manner
func (ip *IncrementalParser) Parse() error {
	refs, err := ip.parser.GetReferences(ip.content)
	if err != nil {
		return err
	}

	ip.mu.Lock()
	ip.references = refs
	ip.mu.Unlock()

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
	ip.parser.CloseParser()
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
