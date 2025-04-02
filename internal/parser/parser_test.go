package parser_test

import (
	"fmt"
	"testing"
	"zeta/internal/parser"
)

func TestParser(t *testing.T) {
	test := []byte(`
    == Hello World
    - very
    @bar
    - nice
    
    [[Hello]] world [[foo]] [][]]][[[ [[Bar]]
    $ a = b $ foo [[ hello ]] hello[[hello]]

    #link("foo")

    http://hello.world

    #foo(bar)

    @foo
    Hello @baz

    Heyyy hhoo this is normal content
    `)
	p, err := parser.NewParser(test)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	query := []byte(`
    (code (call
      item: (ident) @link (#eq? @link "link")
      (group (string) @target ))
    )
    (url) @target
	`)

	results, _ := p.Parse(query)
	for _, r := range results {
		fmt.Println(r)
	}
}

// func TestParserPool(t *testing.T) {
// 	pp := parser.NewParserPool(2, parser.NewParser().parser.Language())
// 	defer pp.Close()
//
// 	document := []byte("some_typst_code")
// 	query := []byte("(some_query)")
//
// 	captures, err := pp.Parse(document, query)
// 	assert.NoError(t, err)
// 	assert.NotNil(t, captures)
// }
