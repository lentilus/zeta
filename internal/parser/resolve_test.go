package parser_test

import (
	"testing"
	"zeta/internal/parser"
)

func TestResolve(t *testing.T) {
	if res, err := parser.Resolve("foo/bar.typ", "baz"); res != "baz.typ" || err != nil {
		t.Fatal()
	}
	if res, err := parser.Resolve("foo/bar.typ", "./Hello_World/../lulu"); res != "foo/lulu.typ" ||
		err != nil {
		t.Fatal()
	}
	if res, err := parser.Resolve("bar/lulufoo/foo.typ", "./foo.txt"); res != "bar/lulufoo/foo.txt" ||
		err != nil {
		t.Fatal()
	}
}
