package index

import (
	"aftermath/bindings"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// Get zettel references by a treesitter query from file content.
func GetReferences(content []byte, refQuery []byte) ([]string, error) {
	parser := sitter.NewParser()
	defer parser.Close()

	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)

	// Parse the source code
	tree := parser.Parse(nil, content)
	defer tree.Close()

	// Query the tree
	query, err := sitter.NewQuery([]byte(refQuery), lang)
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

func main() {
	query := `(
  (ref) @reference
)`
	content := ` @foo
    hello`

	fmt.Print(GetReferences([]byte(content), []byte(query)))
}
