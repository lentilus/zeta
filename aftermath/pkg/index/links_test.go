package index_test

import (
	"aftermath/pkg/index"
	"testing"
)

func TestLink2File(t *testing.T) {
	testIndexer := index.NewIndexer(nil, "/base/dir")
	tests := []struct {
		input    string
		expected string
	}{
		{"foo.bar", "/base/dir/foo/bar.typ"},
		{"foo.bar.baz", "/base/dir/foo/bar/baz.typ"},
		{"foo_bar", "/base/dir/foo_bar.typ"},
		{"foo-bar", "/base/dir/foo-bar.typ"},
		{"foo.bar-baz.qux", "/base/dir/foo/bar-baz/qux.typ"},
	}

	for _, test := range tests {
		result := testIndexer.Link2File(test.input)
		if result != test.expected {
			t.Errorf("link2File(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}
