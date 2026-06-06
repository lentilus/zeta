package server

import (
	"strings"
	"zeta/internal/resolver"
	"zeta/internal/sitteradapter"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// textDocumentCompletion offers, while the cursor is inside a link target,
// existing notes searchable by title plus a "create note" item. The accepted
// item inserts the note's filename reference (so the user never types it) and,
// when configured, fills the link's display body with the title.
func (s *Server) textDocumentCompletion(
	context *glsp.Context,
	params *protocol.CompletionParams,
) (any, error) {
	note, err := resolver.Resolve(params.TextDocument.URI)
	if err != nil {
		return nil, nil
	}
	doc, err := s.manager.GetDocument(note.URI)
	if err != nil {
		return nil, nil
	}
	src := string(doc)

	nodes, err := s.parsers.ParseAndQuery(doc, []byte(s.config.Query))
	if err != nil {
		return nil, nil
	}

	cursorIdx := params.Position.IndexIn(src)
	target := targetAt(nodes["target"], cursorIdx)
	if target == nil {
		return nil, nil // not inside a link target
	}

	editRange, typed := referenceEdit(target, src)

	var displayEdit func(title string) []protocol.TextEdit
	if s.config.CompletionInsertDisplay {
		displayEdit = s.displayBodyEditor(target, src)
	}

	refKind := protocol.CompletionItemKindReference
	items := []protocol.CompletionItem{}
	exactMatch := false

	count := 0
	for _, p := range s.cache.GetPaths() {
		if p == note.CachePath {
			continue // don't suggest linking a note to itself
		}
		meta, _ := s.cache.GetMetaData(p)
		title := resolver.Title(p, meta)
		if !isSubsequence(typed, title) {
			continue
		}
		if strings.EqualFold(title, typed) {
			exactMatch = true
		}
		detail := p
		item := protocol.CompletionItem{
			Label:      title,
			Kind:       &refKind,
			Detail:     &detail,
			FilterText: &title,
			TextEdit:   protocol.TextEdit{Range: editRange, NewText: resolver.Reference(p)},
		}
		if displayEdit != nil {
			item.AdditionalTextEdits = displayEdit(title)
		}
		items = append(items, item)
		if count++; count >= 200 {
			break
		}
	}

	// Offer to create a new note when the typed concept matches nothing exactly.
	if t := strings.TrimSpace(typed); t != "" && !exactMatch {
		if ref, newNote, gerr := s.generateNewNote(t); gerr == nil {
			createKind := protocol.CompletionItemKindFile
			detail := "create " + newNote.CachePath
			sortLast := "￿" // keep the create item below real matches
			item := protocol.CompletionItem{
				Label:      "Create note: " + t,
				Kind:       &createKind,
				Detail:     &detail,
				FilterText: &typed,
				SortText:   &sortLast,
				TextEdit:   protocol.TextEdit{Range: editRange, NewText: ref},
				// Eager creation on clients that run completion commands (e.g.
				// Neovim). Helix ignores this; the file is created lazily on
				// navigation or via the code action instead.
				Command: &protocol.Command{
					Title:     "Create note",
					Command:   "createNote",
					Arguments: []any{newNote.CachePath, t},
				},
			}
			if displayEdit != nil {
				item.AdditionalTextEdits = displayEdit(t)
			}
			items = append(items, item)
		}
	}

	// IsIncomplete asks the client to recompute as the user keeps typing, so the
	// title filter and the "create" item stay current rather than frozen.
	return protocol.CompletionList{IsIncomplete: true, Items: items}, nil
}

// textDocumentCodeAction offers "Create note" on a link whose target does not
// exist yet, giving eager creation on clients (such as Helix) that do not run
// completion-item commands.
func (s *Server) textDocumentCodeAction(
	context *glsp.Context,
	params *protocol.CodeActionParams,
) (any, error) {
	note, err := resolver.Resolve(params.TextDocument.URI)
	if err != nil {
		return nil, nil
	}
	doc, err := s.manager.GetDocument(note.URI)
	if err != nil {
		return nil, nil
	}
	src := string(doc)
	nodes, err := s.parsers.ParseAndQuery(doc, []byte(s.config.Query))
	if err != nil {
		return nil, nil
	}

	startIdx := params.Range.Start.IndexIn(src)
	endIdx := params.Range.End.IndexIn(src)
	quickfix := protocol.CodeActionKind("quickfix")

	var actions []protocol.CodeAction
	for _, n := range nodes["target"] {
		ns, ne := int(n.StartByte()), int(n.EndByte())
		if endIdx < ns || startIdx > ne {
			continue // selection does not touch this link
		}
		tgt, rerr := resolver.ResolveReference(note, n.Content(doc))
		if rerr != nil || s.cache.NoteExists(tgt.CachePath) {
			continue
		}
		title := tgt.CachePath
		if body, ok := linkBodyText(n, doc); ok && strings.TrimSpace(body) != "" {
			title = strings.TrimSpace(body)
		}
		actions = append(actions, protocol.CodeAction{
			Title: "Create note: " + title,
			Kind:  &quickfix,
			Command: &protocol.Command{
				Title:     "Create note",
				Command:   "createNote",
				Arguments: []any{tgt.CachePath, title},
			},
		})
	}
	if len(actions) == 0 {
		return nil, nil
	}
	return actions, nil
}

// targetAt returns the target node that contains the given byte offset.
func targetAt(targets []*sitter.Node, cursorIdx int) *sitter.Node {
	for _, n := range targets {
		if cursorIdx >= int(n.StartByte()) && cursorIdx <= int(n.EndByte()) {
			return n
		}
	}
	return nil
}

// referenceEdit computes the document range covering the reference substring of
// a target node (the part select_regex extracts) and the text currently there.
func referenceEdit(target *sitter.Node, src string) (protocol.Range, string) {
	content := target.Content([]byte(src))
	rs, re, ok := resolver.SelectReferenceRange(content)
	if !ok {
		// Partial/unterminated string: replace everything after the opening
		// delimiter up to the cursor's node end.
		rs = 1
		if rs > len(content) {
			rs = len(content)
		}
		re = rs
	}
	row := target.StartPoint().Row
	col := target.StartPoint().Column
	start := sitteradapter.TSPointToLSPPosition(sitter.Point{Row: row, Column: col + uint32(rs)}, src)
	end := sitteradapter.TSPointToLSPPosition(sitter.Point{Row: row, Column: col + uint32(re)}, src)
	typed := ""
	if rs <= re && re <= len(content) {
		typed = content[rs:re]
	}
	return protocol.Range{Start: start, End: end}, typed
}

// displayBodyEditor returns a function producing the additionalTextEdits that
// fill a link's display body with a title. It returns nil when there is nothing
// to do (an existing non-empty body is never overwritten).
func (s *Server) displayBodyEditor(target *sitter.Node, src string) func(string) []protocol.TextEdit {
	linkCall := enclosingCall(target)
	if linkCall == nil {
		return nil
	}
	if body := bodyOf(linkCall); body != nil {
		open := body.Child(0)
		closeN := body.Child(int(body.ChildCount()) - 1)
		if open == nil || closeN == nil {
			return nil
		}
		innerStart := sitteradapter.TSPointToLSPPosition(open.EndPoint(), src)
		innerEnd := sitteradapter.TSPointToLSPPosition(closeN.StartPoint(), src)
		if innerStart != innerEnd {
			return nil // body already has content
		}
		return func(title string) []protocol.TextEdit {
			return []protocol.TextEdit{{
				Range:   protocol.Range{Start: innerStart, End: innerEnd},
				NewText: title,
			}}
		}
	}
	// No body present: insert one right after the link call.
	at := sitteradapter.TSPointToLSPPosition(linkCall.EndPoint(), src)
	return func(title string) []protocol.TextEdit {
		return []protocol.TextEdit{{
			Range:   protocol.Range{Start: at, End: at},
			NewText: "[" + title + "]",
		}}
	}
}

// linkBodyText returns the text of a link's display body, if present.
func linkBodyText(target *sitter.Node, doc []byte) (string, bool) {
	linkCall := enclosingCall(target)
	if linkCall == nil {
		return "", false
	}
	body := bodyOf(linkCall)
	if body == nil {
		return "", false
	}
	open := body.Child(0)
	closeN := body.Child(int(body.ChildCount()) - 1)
	if open == nil || closeN == nil {
		return "", false
	}
	s, e := int(open.EndByte()), int(closeN.StartByte())
	if s < 0 || e > len(doc) || s > e {
		return "", false
	}
	return string(doc[s:e]), true
}

// enclosingCall returns the nearest ancestor "call" node (the link call).
func enclosingCall(n *sitter.Node) *sitter.Node {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		if cur.Type() == "call" {
			return cur
		}
	}
	return nil
}

// bodyOf returns the content block ("[...]") of a link call, if any. It is the
// "content" sibling of the link call within an enclosing call node.
func bodyOf(linkCall *sitter.Node) *sitter.Node {
	parent := linkCall.Parent()
	if parent == nil || parent.Type() != "call" {
		return nil
	}
	for i := 0; i < int(parent.ChildCount()); i++ {
		if c := parent.Child(i); c != nil && c.Type() == "content" {
			return c
		}
	}
	return nil
}
