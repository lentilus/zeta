package index_test

import (
	"aftermath/pkg/index"
	"reflect"
	"testing"
)

var query = `
(
  (ref) @reference
)
`
var contentA = `
= Hello World
- @foo, @bar

And one more @foo
`
var refsA = []string{"@foo", "@bar", "@foo"}

var contentB = `No refs here`
var refsB = []string{}

func TestGetReferences(t *testing.T) {
	// Test Case A
	resA, err := index.GetReferences([]byte(contentA), []byte(query))

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resA, refsA) {
		t.Errorf("Expected %v, got %v", refsA, resA)
	}

	// Test Case B
	resB, err := index.GetReferences([]byte(contentB), []byte(query))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resB, refsB) {
		t.Errorf("Expected %v, got %v", refsB, resB)
	}

}
