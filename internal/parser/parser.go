package parser

import (
	"context"
	"fmt"
	"sync"

	typst "zeta/tree-sitter-typst"

	sitter "github.com/smacker/go-tree-sitter"
)

var (
	lang        = sitter.NewLanguage(typst.Language())
	captureName = "target"
)

type Match struct {
	Row     uint32
	Col     uint32
	Content string
}

// Edit represents an edit change for incremental parsing.
type Edit sitter.EditInput

func executeQuery(
	root *sitter.Node,
	query []byte,
	lang *sitter.Language,
	source []byte,
) ([]Match, error) {
	q, err := sitter.NewQuery(query, lang)
	if err != nil {
		return nil, err
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, root)

	var matches []Match

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		m = qc.FilterPredicates(m, source)

		for _, c := range m.Captures {
			name := q.CaptureNameForId(c.Index)
			if name != captureName {
				continue
			}
			match := Match{
				Row:     c.Node.StartPoint().Row,
				Col:     c.Node.StartPoint().Column,
				Content: c.Node.Content(source),
			}

			matches = append(matches, match)
		}
	}

	return matches, nil
}

// Parser wraps a tree-sitter parser instance along with a (possibly) stateful syntax tree.
type Parser struct {
	parser *sitter.Parser
	tree   *sitter.Tree
	source []byte
	mu     sync.Mutex
}

// NewParser creates a new Parser using the default language and parses
// the provided initialText if it is non-empty. It returns the Parser or an error.
func NewParser(initialText []byte) (*Parser, error) {
	p := sitter.NewParser()
	p.SetLanguage(lang)
	parser := &Parser{
		parser: p,
		source: initialText,
	}
	if len(initialText) > 0 {
		tree, err := p.ParseCtx(context.Background(), nil, initialText)
		if err != nil {
			return nil, fmt.Errorf("failed to parse initial text: %w", err)
		}
		parser.tree = tree
	}
	return parser, nil
}

// Parse runs the provided query against the previously parsed tree, applying predicate filtering.
func (p *Parser) Parse(query []byte) ([]Match, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tree == nil {
		return nil, fmt.Errorf("no parsed tree available; first parse a document")
	}
	return executeQuery(p.tree.RootNode(), query, lang, p.source)
}

// Update applies a set of changes (edits) to the currently parsed tree.
func (p *Parser) Update(changes []Edit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tree == nil {
		return fmt.Errorf("no tree available to update")
	}
	for _, change := range changes {
		p.tree.Edit(sitter.EditInput(change))
	}
	return nil
}

// Close frees any resources held by the Parser.
func (p *Parser) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tree != nil {
		p.tree.Close()
		p.tree = nil
	}
	if p.parser != nil {
		p.parser.Close()
		p.parser = nil
	}
	return nil
}

// ParserPool maintains a pool of Parser instances for one-time parsing.
type ParserPool struct {
	pool chan *Parser
	lang *sitter.Language
}

// NewParserPool creates a ParserPool with n Parser instances for the specified language.
func NewParserPool(n int, lang *sitter.Language) *ParserPool {
	pp := &ParserPool{
		pool: make(chan *Parser, n),
		lang: lang,
	}
	for i := 0; i < n; i++ {
		parser, err := NewParser([]byte{})
		if err != nil {
			// Handle error appropriately, e.g., panic or return error
			panic(fmt.Sprintf("failed to create parser: %v", err))
		}
		pp.pool <- parser
	}
	return pp
}

// Parse performs a one-time parse of the document using one Parser from the pool.
// It creates a new syntax tree from the document, runs the provided query (with predicate filtering)
// and returns all matches.
func (pp *ParserPool) Parse(document []byte, query []byte) ([]Match, error) {
	// Acquire a parser from the pool.
	p := <-pp.pool
	defer func() { pp.pool <- p }()

	tree, err := p.parser.ParseCtx(context.Background(), nil, document)
	if err != nil {
		return nil, err
	}
	// Update the Parser with the new tree and source.
	p.mu.Lock()
	p.tree = tree
	p.source = document
	p.mu.Unlock()

	return executeQuery(tree.RootNode(), query, pp.lang, document)
}

// Close releases all Parser instances in the pool.
func (pp *ParserPool) Close() error {
	close(pp.pool)
	for p := range pp.pool {
		p.Close()
	}
	return nil
}
