package parser

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func doc(text string) []byte {
	return []byte(text)
}

// mapKeys returns the keys of a map for diagnostic messages.
func mapKeys(m map[string][]*sitter.Node) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestNewParser(t *testing.T) {
	p, err := NewParser()
	if err != nil {
		t.Fatalf("NewParser() unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("NewParser() returned nil Parser")
	}
	p.Close()
}

func TestParse(t *testing.T) {
	p, err := NewParser()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if err := p.Parse(doc("Hello world")); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if err := p.Parse(doc("= Heading")); err != nil {
		t.Fatalf("Parse() error on second doc: %v", err)
	}
}

func TestQuery(t *testing.T) {
	p, err := NewParser()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	src := doc("Hello *world*!")
	if err := p.Parse(src); err != nil {
		t.Fatal(err)
	}

	query := []byte("(strong) @strong")
	result, err := p.Query(query, src)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}

	nodes, ok := result["strong"]
	if !ok {
		t.Fatalf("Query() missing capture name 'strong', got keys: %v", mapKeys(result))
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 'strong' node, got %d", len(nodes))
	}
	// The strong node includes the delimiter characters.
	if nodes[0].Content(src) != "*world*" {
		t.Fatalf("expected content '*world*', got '%s'", nodes[0].Content(src))
	}
}

func TestQueryNoTree(t *testing.T) {
	p, err := NewParser()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	_, err = p.Query([]byte("(text) @t"), doc("test"))
	if err == nil {
		t.Fatal("expected error when no tree is available")
	}
	if !strings.Contains(err.Error(), "no parsed tree available") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestUpdateAndQuery(t *testing.T) {
	p, err := NewParser()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	original := doc("Hello world")
	if err := p.Parse(original); err != nil {
		t.Fatal(err)
	}

	// Insert " *strong*" at the end.
	insertion := []byte(" *strong*")
	newDoc := append(original, insertion...)

	edit := sitter.EditInput{
		StartIndex:  uint32(len(original)),
		OldEndIndex: uint32(len(original)),
		NewEndIndex: uint32(len(newDoc)),
		StartPoint: sitter.Point{
			Row:    0,
			Column: uint32(len(original)),
		},
		OldEndPoint: sitter.Point{
			Row:    0,
			Column: uint32(len(original)),
		},
		NewEndPoint: sitter.Point{
			Row:    0,
			Column: uint32(len(newDoc)),
		},
	}
	if err := p.Update(edit); err != nil {
		t.Fatal(err)
	}

	if err := p.Parse(newDoc); err != nil {
		t.Fatal(err)
	}

	query := []byte("(strong) @strong")
	result, err := p.Query(query, newDoc)
	if err != nil {
		t.Fatal(err)
	}
	nodes, ok := result["strong"]
	if !ok || len(nodes) != 1 {
		t.Fatalf("expected 1 'strong' node after update, got %v", result)
	}
}

func TestParserPool_ParseAndQuery(t *testing.T) {
	pool := NewParserPool(2)
	defer pool.Close()

	src1 := doc("Hello *world*")
	src2 := doc("= Heading *bold*")
	query := []byte("(strong) @s")

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		res, err := pool.ParseAndQuery(src1, query)
		if err != nil {
			errs <- err
			return
		}
		nodes := res["s"]
		if len(nodes) != 1 || nodes[0].Content(src1) != "*world*" {
			errs <- fmt.Errorf("doc1: expected 1 strong node with content '*world*', got %v", res)
			return
		}
	}()

	go func() {
		defer wg.Done()
		res, err := pool.ParseAndQuery(src2, query)
		if err != nil {
			errs <- err
			return
		}
		nodes := res["s"]
		if len(nodes) != 1 || nodes[0].Content(src2) != "*bold*" {
			errs <- fmt.Errorf("doc2: expected 1 strong node with content '*bold*', got %v", res)
			return
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestParser_Close(t *testing.T) {
	p, err := NewParser()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(doc("test")); err != nil {
		t.Fatal(err)
	}

	// Close must succeed.
	if err := p.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// After Close, Parse would dereference a nil parser and panic.
	// The module currently does not guard against this, so we skip further calls.
}

func TestParserPool_Close(t *testing.T) {
	pool := NewParserPool(1)
	// Acquire the only parser so the pool channel is empty.
	parser := <-pool.pool
	// Close the pool while one parser is still out.
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	// The parser we removed is still valid; we must close it ourselves.
	parser.Close()
}
