package parser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"runtime"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

var (
	captureName = "target"
	configured bool = false
	formats map[string]Format = make(map[string]Format)
	regexes map[string]*regexp.Regexp = make(map[string]*regexp.Regexp)
)

// Format encapsulates the all info that the parser needs to work with a
// document format.
type Format struct {
	Query              string   `json:"query"`
	SelectRegex        string   `json:"select_regex"`
	Extensions         []string `json:"extensions"`
	DefaultExtension   string   `json:"default_extension"`
	TitleTemplate      string   `json:"title_template"`
	TitleSubstitutions []string `json:"title_substitutions"`
}

// Parser wraps a tree-sitter parser instance, with its language and syntax tree
// as well as the config for the language format.
type Parser struct{
	parser *sitter.Parser
	tree   *sitter.Tree
	lang   *sitter.Language
	format *Format
	mu     sync.Mutex
}

func AddFormat(language string, format Format) {
	formats[language] = format
}


// NewParser creates a new parser for the language
func NewParser(language string) (*Parser, error) {
	log.Printf("Creating new parser for %s", language)
    lang, ok := languages[language]
	if !ok {
		return nil, errors.New("No parser for this language.")
	}

	format, ok := formats[language] 
	if !ok {
		return nil, errors.New("No format for this language")
	}

	p := sitter.NewParser()
	p.SetLanguage(lang)
	parser := &Parser{
		parser: p,
		lang: lang,
		format: &format,
	}

	// I believe we should not need this, something is off
	runtime.KeepAlive(lang)

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
func (p *Parser) query(query []byte, document []byte) (map[string][]*sitter.Node, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tree == nil {
		return nil, fmt.Errorf("no parsed tree available; first parse a document")
	}
	return executeQuery(p.tree.RootNode(), p.lang, query, document)
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



func executeQuery(
	root *sitter.Node,
	lang *sitter.Language,
	query []byte,
	document []byte,
) (map[string][]*sitter.Node, error) {
	q, err := sitter.NewQuery(query, lang)
	if err != nil {
		return nil, err
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, root)

	nodes := make(map[string][]*sitter.Node)
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
		captureName := q.CaptureNameForId(c.Index)
		nodes[captureName] = append(nodes[captureName], c.Node)
	}
	return nodes, nil
}
