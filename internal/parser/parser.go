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

// Parser wraps a tree-sitter parser instance along with a (possibly) stateful syntax tree.
type Parser struct {
	parser *sitter.Parser
	tree   *sitter.Tree
	mu     sync.Mutex
}

// NewParser creates a new Parser using the default language and parses
// the provided initialText if it is non-empty. It returns the Parser or an error.
func NewParser() (*Parser, error) {
	p := sitter.NewParser()
	p.SetLanguage(lang)
	parser := &Parser{
		parser: p,
	}
	return parser, nil
}

func (p *Parser) Parse(document []byte) error {
	// Do a full parse of the document
	tree, err := p.parser.ParseCtx(context.Background(), p.tree, document)
	if err != nil {
		return err
	}
	// Update the Parser with the new tree
	p.mu.Lock()
	p.tree = tree
	p.mu.Unlock()
	return nil
}

// Query runs the provided query against the previously parsed tree, applying predicate filtering.
func (p *Parser) Query(query []byte, document []byte) ([]*sitter.Node, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tree == nil {
		return nil, fmt.Errorf("no parsed tree available; first parse a document")
	}
	return executeQuery(p.tree.RootNode(), lang, query, document)
}

func (p *Parser) Update(edit sitter.EditInput) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tree == nil {
		return fmt.Errorf("no tree available to update")
	}

	p.tree.Edit(edit)
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
	for range n {
		parser, err := NewParser()
		if err != nil {
			panic(fmt.Sprintf("failed to create parser: %v", err))
		}
		pp.pool <- parser
	}
	return pp
}

// Parse performs a one-time parse of the document using one Parser from the pool.
// It creates a new syntax tree from the document, runs the provided query (with predicate filtering)
// and returns all matches.
func (pp *ParserPool) ParseAndQuery(document []byte, query []byte) ([]*sitter.Node, error) {
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
	p.mu.Unlock()

	return executeQuery(tree.RootNode(), pp.lang, query, document)
}

// Close releases all Parser instances in the pool.
func (pp *ParserPool) Close() error {
	close(pp.pool)
	for p := range pp.pool {
		p.Close()
	}
	return nil
}

func executeQuery(
	root *sitter.Node,
	lang *sitter.Language,
	query []byte,
	document []byte,
) ([]*sitter.Node, error) {
	q, err := sitter.NewQuery(query, lang)
	if err != nil {
		return nil, err
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, root)

	var nodes []*sitter.Node
	var captures []sitter.QueryCapture

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		m = qc.FilterPredicates(m, document)
		captures = append(captures, m.Captures...)
	}

	for _, c := range captures {
		if q.CaptureNameForId(c.Index) != captureName {
			continue
		}
		nodes = append(nodes, c.Node)
	}

	return nodes, nil
}
