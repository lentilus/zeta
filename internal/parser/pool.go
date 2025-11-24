package parser

import (
	"context"
	"fmt"
	"zeta/internal/cache"
	"zeta/internal/resolver"

)

// ParserPool maintains a pool of Parser instances for one-time parsing.
type ParserPool struct {
	pool chan *Parser
}

// NewParserPool creates a ParserPool with n Parser instances for the specified language.
func NewParserPool(n int, language string) *ParserPool {
	pp := &ParserPool{
		pool: make(chan *Parser, n),
	}
	for range n {
		parser, err := NewParser(language)
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
func (pp *ParserPool) ParseAndExtractLinksAndMeta(
	note resolver.Note,
	document []byte,
) ([]cache.Link, map[string]string) {
	// Acquire a parser from the pool.
	p := <-pp.pool
	defer func() { pp.pool <- p }()

	tree, err := p.parser.ParseCtx(context.Background(), nil, document)
	if err != nil {
		panic(err)
		return nil, nil
	}
	// Update the Parser with the new tree and source.
	p.mu.Lock()
	p.tree = tree
	p.mu.Unlock()

	// return executeQuery(tree.RootNode(), p.lang, []byte(p.format.Query), document)
	return p.ExtractLinksAndMeta(note, document)
}

// Close releases all Parser instances in the pool.
func (pp *ParserPool) Close() error {
	close(pp.pool)
	for p := range pp.pool {
		p.Close()
	}
	return nil
}
