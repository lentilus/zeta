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

	// plan describes the single text edit each item performs. The display body
	// is folded into this one edit (rather than additionalTextEdits) because
	// some clients (Helix) apply additional edits against the post-edit buffer
	// without rebasing offsets, which corrupts edits positioned after the
	// reference on the same line.
	plan, typed := s.completionPlan(target, src)

	refKind := protocol.CompletionItemKindReference
	snippetFormat := protocol.InsertTextFormatSnippet
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
		edit, isSnippet := plan.edit(resolver.Reference(p), resolver.Display(p, meta))
		item := protocol.CompletionItem{
			Label:      title,
			Kind:       &refKind,
			Detail:     &detail,
			FilterText: &title,
			TextEdit:   edit,
		}
		if isSnippet {
			item.InsertTextFormat = &snippetFormat
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
			edit, isSnippet := plan.edit(ref, t)
			item := protocol.CompletionItem{
				Label:      "Create note: " + t,
				Kind:       &createKind,
				Detail:     &detail,
				FilterText: &typed,
				SortText:   &sortLast,
				TextEdit:   edit,
				// Eager creation on clients that run completion commands (e.g.
				// Neovim). Helix ignores this; the file is created lazily on
				// navigation or via the code action instead.
				Command: &protocol.Command{
					Title:     "Create note",
					Command:   "createNote",
					Arguments: []any{newNote.CachePath, t},
				},
			}
			if isSnippet {
				item.InsertTextFormat = &snippetFormat
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

// bodyPlan describes how each completion item rewrites the link in a single
// text edit. refRange/refEnd cover just the reference substring; when fill is
// set, fillRange additionally extends through the display body, with middle
// being the verbatim text (e.g. `")` ) between the reference and the body so it
// can be reproduced, and bracket indicating whether the body must be wrapped in
// new `[...]` (no body present) or inserted into an existing empty one.
type bodyPlan struct {
	refRange  protocol.Range
	fill      bool
	fillRange protocol.Range
	middle    string
	bracket   bool
	snippet   bool // client supports snippet placeholders
}

// edit builds the text edit for an item given its reference and display text.
// The second return value reports whether the edit is a snippet (so the caller
// can set InsertTextFormat). When the client supports snippets the display body
// is wrapped in a `${1:…}` tab stop, leaving it selected after acceptance.
func (p bodyPlan) edit(ref, display string) (protocol.TextEdit, bool) {
	if !p.fill || display == "" {
		return protocol.TextEdit{Range: p.refRange, NewText: ref}, false
	}
	if p.snippet {
		body := "${1:" + snippetEscape(display) + "}"
		if p.bracket {
			body = "[" + body + "]"
		}
		newText := snippetEscape(ref) + snippetEscape(p.middle) + body
		return protocol.TextEdit{Range: p.fillRange, NewText: newText}, true
	}
	body := display
	if p.bracket {
		body = "[" + display + "]"
	}
	return protocol.TextEdit{Range: p.fillRange, NewText: ref + p.middle + body}, false
}

// snippetEscape escapes the characters that are special in LSP snippet syntax
// so literal text is inserted verbatim.
func snippetEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `$`, `\$`)
	s = strings.ReplaceAll(s, `}`, `\}`)
	return s
}

// completionPlan computes the single-edit plan for the link target under the
// cursor and returns the reference text currently typed.
func (s *Server) completionPlan(target *sitter.Node, src string) (bodyPlan, string) {
	content := target.Content([]byte(src))
	rs, re, ok := resolver.SelectReferenceRange(content)
	if !ok {
		// Partial/unterminated string: replace everything after the opening
		// delimiter, and do not attempt to fill a body.
		rs = 1
		if rs > len(content) {
			rs = len(content)
		}
		re = rs
	}
	row := target.StartPoint().Row
	col := target.StartPoint().Column
	refStart := sitteradapter.TSPointToLSPPosition(sitter.Point{Row: row, Column: col + uint32(rs)}, src)
	refEnd := sitteradapter.TSPointToLSPPosition(sitter.Point{Row: row, Column: col + uint32(re)}, src)
	typed := ""
	if rs <= re && re <= len(content) {
		typed = content[rs:re]
	}
	plan := bodyPlan{
		refRange: protocol.Range{Start: refStart, End: refEnd},
		snippet:  s.supportsSnippets,
	}

	if !ok || !s.config.CompletionInsertDisplay {
		return plan, typed
	}
	linkCall := enclosingCall(target)
	if linkCall == nil {
		return plan, typed
	}

	refEndByte := int(target.StartByte()) + re
	var pointByte int
	var point sitter.Point
	if body := bodyOf(linkCall); body != nil {
		open := body.Child(0)
		closeN := body.Child(int(body.ChildCount()) - 1)
		if open == nil || closeN == nil || open.EndByte() != closeN.StartByte() {
			return plan, typed // missing brackets or an existing non-empty body
		}
		pointByte, point = int(open.EndByte()), open.EndPoint()
		plan.bracket = false
	} else {
		pointByte, point = int(linkCall.EndByte()), linkCall.EndPoint()
		plan.bracket = true
	}
	if pointByte < refEndByte || pointByte > len(src) {
		return plan, typed
	}
	bodyPos := sitteradapter.TSPointToLSPPosition(point, src)
	if bodyPos.Line != refStart.Line {
		return plan, typed // body on another line; keep it simple
	}
	plan.fill = true
	plan.fillRange = protocol.Range{Start: refStart, End: bodyPos}
	plan.middle = src[refEndByte:pointByte]
	return plan, typed
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
