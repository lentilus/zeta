package server

import (
	"os"
	"path/filepath"
	"testing"

	"zeta/internal/cache"
	"zeta/internal/config"
	"zeta/internal/manager"
	"zeta/internal/parser"
	"zeta/internal/resolver"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const demoQuery = `
	(code (call item: (call item: (ident) @link (#eq? @link "link") (group (string) @target ))))
	(heading (text) @title)
	(heading (label) @taxon)
`

func newTestServer(t *testing.T, root string) *Server {
	t.Helper()
	if err := resolver.Configure(root, `^"(.*)"$`, []string{".typ"}, ".typ", "%s %s", []string{"taxon", "title"}); err != nil {
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
			NewNoteIDScheme:         "random",
			NewNoteTemplate:         "= %s\n",
		},
	}
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

func TestCompletionExistingNoteInsertsReferenceAndDisplay(t *testing.T) {
	root := t.TempDir()
	s := newTestServer(t, root)
	// Seed an existing, non-placeholder note.
	if err := s.cache.EditNote("axiom.typ", nil, map[string]string{"title": "Axiom of Choice"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	uri := openDoc(t, s, root, "source.typ", "#link(\"\")[]\n")
	items := complete(t, s, uri, 0, 7) // cursor between the quotes

	var axiom *protocol.CompletionItem
	for i := range items {
		if items[i].Label == "Axiom of Choice" {
			axiom = &items[i]
		}
	}
	if axiom == nil {
		t.Fatalf("expected an 'Axiom of Choice' completion, got %d items", len(items))
	}

	edit := axiom.TextEdit.(protocol.TextEdit)
	if edit.NewText != "axiom" {
		t.Errorf("target edit = %q, want %q", edit.NewText, "axiom")
	}
	if edit.Range.Start != (protocol.Position{Line: 0, Character: 7}) {
		t.Errorf("target edit start = %+v, want {0,7}", edit.Range.Start)
	}
	if len(axiom.AdditionalTextEdits) != 1 {
		t.Fatalf("expected 1 display edit, got %d", len(axiom.AdditionalTextEdits))
	}
	disp := axiom.AdditionalTextEdits[0]
	if disp.NewText != "Axiom of Choice" {
		t.Errorf("display edit = %q, want %q", disp.NewText, "Axiom of Choice")
	}
	if disp.Range.Start != (protocol.Position{Line: 0, Character: 10}) {
		t.Errorf("display edit start = %+v, want {0,10}", disp.Range.Start)
	}
}

func TestCompletionOffersCreateForUnknownConcept(t *testing.T) {
	root := t.TempDir()
	s := newTestServer(t, root)

	uri := openDoc(t, s, root, "source.typ", "#link(\"quantum\")[]\n")
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
	// Display fill uses the typed concept.
	if len(create.AdditionalTextEdits) != 1 || create.AdditionalTextEdits[0].NewText != "quantum" {
		t.Errorf("create display edit = %v, want fill 'quantum'", create.AdditionalTextEdits)
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
