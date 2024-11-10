package parser

import (
	"aftermath/bindings"

	sitter "github.com/smacker/go-tree-sitter"
)

var refQuery = []byte(`((ref) @reference)`)

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
