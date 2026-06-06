package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeta/internal/cache"
	"zeta/internal/config"
	"zeta/internal/manager"
	"zeta/internal/parser"
	"zeta/internal/resolver"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const demoQuery = `
	(code (call item: (ident) @link (#eq? @link "link") (group (string) @target )))
	(code (call item: (call item: (ident) @link (#eq? @link "link") (group (string) @target ))))
	(heading (text) @title)
	(heading (label) @taxon)
`

func newTestServer(t *testing.T, root string) *Server {
	t.Helper()
	if err := resolver.Configure(root, `^"(.*)"$`, []string{".typ"}, ".typ", "%s %s", []string{"taxon", "title"}, "%s", []string{"title"}); err != nil {
		t.Fatalf("configure: %v", err)
	}
	return &Server{
		cache:   cache.NewCache(),
		manager: manager.NewDocumentManager(),
		parsers: parser.NewParserPool(2),
		config: config.Config{
			Query:                   demoQuery,
			DefaultExtension:        ".typ",
			CompletionInsertDisplay: true,
			DisplayTemplate:         "%s",
			DisplaySubstitutions:    []string{"title"},
			NewNoteIDScheme:         "random",
			NewNoteTemplate:         "= %s\n",
		},
	}
}

// applyEdit applies a single completion text edit to src (edits are single-line
// and self-contained now, so this mirrors how a client applies them).
func applyEdit(src string, te protocol.TextEdit) string {
	s := te.Range.Start.IndexIn(src)
	e := te.Range.End.IndexIn(src)
	return src[:s] + te.NewText + src[e:]
}

func openDoc(t *testing.T, s *Server, root, name, src string) string {
	t.Helper()
	uri := "file://" + filepath.Join(root, name)
	note, err := resolver.Resolve(uri)
	if err != nil {
		t.Fatalf("resolve %s: %v", uri, err)
	}
	s.manager.UpdateDocument(note.URI, []byte(src))
	return uri
}

func complete(t *testing.T, s *Server, uri string, line, char uint32) []protocol.CompletionItem {
	t.Helper()
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: line, Character: char},
		},
	}
	res, err := s.textDocumentCompletion(nil, params)
	if err != nil {
		t.Fatalf("completion: %v", err)
	}
	if res == nil {
		return nil
	}
	return res.(protocol.CompletionList).Items
}

func itemByLabel(items []protocol.CompletionItem, label string) *protocol.CompletionItem {
	for i := range items {
		if items[i].Label == label {
			return &items[i]
		}
	}
	return nil
}

// TestCompletionFoldsBodyIntoSingleEdit reproduces the reported corruption:
// completing a bare `#link("typ")` must yield `#link("typst")[Typst]`, with the
// body after the paren (not inside the URI) and the <Tool> taxon dropped.
func TestCompletionFoldsBodyIntoSingleEdit(t *testing.T) {
	cases := []struct {
		name string
		src  string
		char uint32
		want string
	}{
		{"bare link", "#link(\"typ\")\n", 10, "#link(\"typst\")[Typst]\n"},
		{"empty body", "#link(\"typ\")[]\n", 10, "#link(\"typst\")[Typst]\n"},
		{"empty target", "#link(\"\")[]\n", 7, "#link(\"typst\")[Typst]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			s := newTestServer(t, root)
			if err := s.cache.EditNote("typst.typ", nil, map[string]string{"title": "Typst", "taxon": "<Tool>"}); err != nil {
				t.Fatalf("seed: %v", err)
			}
			uri := openDoc(t, s, root, "source.typ", tc.src)
			items := complete(t, s, uri, 0, tc.char)

			it := itemByLabel(items, "<Tool> Typst") // label keeps the taxon for the picker
			if it == nil {
				t.Fatalf("expected a '<Tool> Typst' completion, got %d items", len(items))
			}
			if len(it.AdditionalTextEdits) != 0 {
				t.Errorf("expected no additionalTextEdits, got %d", len(it.AdditionalTextEdits))
			}
			got := applyEdit(tc.src, it.TextEdit.(protocol.TextEdit))
			if got != tc.want {
				t.Errorf("applied = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCompletionSnippetSelectsBody checks that, when the client supports
// snippets, the display body is wrapped in a ${1:…} placeholder so it gets
// selected after acceptance.
func TestCompletionSnippetSelectsBody(t *testing.T) {
	root := t.TempDir()
	s := newTestServer(t, root)
	s.supportsSnippets = true
	if err := s.cache.EditNote("typst.typ", nil, map[string]string{"title": "Typst", "taxon": "<Tool>"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := "#link(\"typ\")\n"
	uri := openDoc(t, s, root, "source.typ", src)
	items := complete(t, s, uri, 0, 10)

	it := itemByLabel(items, "<Tool> Typst")
	if it == nil {
		t.Fatalf("expected a '<Tool> Typst' completion, got %d items", len(items))
	}
	if it.InsertTextFormat == nil || *it.InsertTextFormat != protocol.InsertTextFormatSnippet {
		t.Fatalf("expected snippet insert format, got %v", it.InsertTextFormat)
	}
	edit := it.TextEdit.(protocol.TextEdit)
	if want := "typst\")[${1:Typst}]"; edit.NewText != want {
		t.Errorf("snippet NewText = %q, want %q", edit.NewText, want)
	}
}

func TestCompletionOffersCreateForUnknownConcept(t *testing.T) {
	root := t.TempDir()
	s := newTestServer(t, root)

	src := "#link(\"quantum\")[]\n"
	uri := openDoc(t, s, root, "source.typ", src)
	items := complete(t, s, uri, 0, 8) // inside "quantum"

	var create *protocol.CompletionItem
	for i := range items {
		if items[i].Command != nil && items[i].Command.Command == "createNote" {
			create = &items[i]
		}
	}
	if create == nil {
		t.Fatalf("expected a create-note item, got %d items", len(items))
	}
	if len(create.Command.Arguments) != 2 || create.Command.Arguments[1] != "quantum" {
		t.Errorf("create command args = %v, want [<path> quantum]", create.Command.Arguments)
	}
	// The single edit inserts a fresh id and fills the body with the concept.
	got := applyEdit(src, create.TextEdit.(protocol.TextEdit))
	if !strings.HasSuffix(got, "[quantum]\n") || !strings.HasPrefix(got, "#link(\"") {
		t.Errorf("applied create = %q, want #link(\"<id>\")[quantum]", got)
	}
}

func TestCreateNoteWritesTemplate(t *testing.T) {
	root := t.TempDir()
	s := newTestServer(t, root)

	if err := s.createNote([]any{"quantum.typ", "Quantum Mechanics"}); err != nil {
		t.Fatalf("createNote: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "quantum.typ"))
	if err != nil {
		t.Fatalf("read created note: %v", err)
	}
	if want := "= Quantum Mechanics\n"; string(got) != want {
		t.Errorf("created content = %q, want %q", got, want)
	}
	// The placeholder should now be promoted to a real note.
	if !s.cache.NoteExists("quantum.typ") {
		t.Errorf("expected quantum.typ to exist in cache after creation")
	}
}
