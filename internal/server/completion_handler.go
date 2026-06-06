package server

import (
	"strings"
	"unicode/utf8"
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
	if target := targetAt(nodes["target"], cursorIdx); target != nil {
		return s.linkTargetCompletions(note, src, target)
	}
	// Outside a link: suggest turning the phrase being typed into a link to an
	// existing note whose title it matches. Skip when editing an existing
	// link's display body, where a nested-link suggestion would be unwanted.
	if s.config.SuggestLinksInText && !cursorInLinkBody(nodes["target"], cursorIdx) {
		return s.proseLinkCompletions(note.CachePath, src, params.Position, cursorIdx)
	}
	return nil, nil
}

// cursorInLinkBody reports whether the cursor sits within the display body
// ("[...]") of any link.
func cursorInLinkBody(targets []*sitter.Node, cursorIdx int) bool {
	for _, n := range targets {
		linkCall := enclosingCall(n)
		if linkCall == nil {
			continue
		}
		if body := bodyOf(linkCall); body != nil {
			if cursorIdx >= int(body.StartByte()) && cursorIdx <= int(body.EndByte()) {
				return true
			}
		}
	}
	return false
}

// linkTargetCompletions offers existing notes (searchable by title) plus a
// "create note" item while the cursor is inside a link target.
func (s *Server) linkTargetCompletions(note resolver.Note, src string, target *sitter.Node) (any, error) {
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

// minPhraseMatch is the shortest matched phrase length that triggers a prose
// link suggestion, to avoid noise on one- or two-letter fragments.
const minPhraseMatch = 3

// proseLinkCompletions suggests linking the phrase being typed to an existing
// note when a trailing run of the current line matches the start of its title.
// Accepting replaces the phrase with a full `#link("id")[Title]`.
func (s *Server) proseLinkCompletions(self string, src string, pos protocol.Position, cursorIdx int) (any, error) {
	lineStart := cursorIdx
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	line := src[lineStart:cursorIdx]

	refKind := protocol.CompletionItemKindReference
	snippetFormat := protocol.InsertTextFormatSnippet
	items := []protocol.CompletionItem{}
	count := 0
	for _, p := range s.cache.GetPaths() {
		if p == self {
			continue
		}
		meta, _ := s.cache.GetMetaData(p)
		// Match against the display name (the concept you actually write in
		// prose, e.g. "Typst"), not the full title which may be prefixed by a
		// classifier such as a "<Tool>" taxon.
		display := resolver.Display(p, meta)
		n, score := bestLinkWindow(line, display)
		if n == 0 || score < s.config.LinkSuggestThreshold {
			continue
		}
		startCol := uint32(len(line) - n) // byte column of the match within the line
		start := sitteradapter.TSPointToLSPPosition(sitter.Point{Row: pos.Line, Column: startCol}, src)

		newText, isSnippet := s.linkInsert(resolver.Reference(p), display)
		detail := "link " + p
		title := resolver.Title(p, meta)
		item := protocol.CompletionItem{
			Label:      title,
			Kind:       &refKind,
			Detail:     &detail,
			FilterText: &display,
			TextEdit:   protocol.TextEdit{Range: protocol.Range{Start: start, End: pos}, NewText: newText},
		}
		if isSnippet {
			item.InsertTextFormat = &snippetFormat
		}
		items = append(items, item)
		if count++; count >= 50 {
			break
		}
	}
	if len(items) == 0 {
		return nil, nil
	}
	return protocol.CompletionList{IsIncomplete: true, Items: items}, nil
}

// linkInsert builds the text that wraps a reference and display title in a link,
// using a selectable snippet placeholder for the body when the client supports
// snippets.
func (s *Server) linkInsert(ref, display string) (string, bool) {
	if s.supportsSnippets {
		return `#link("` + snippetEscape(ref) + `")[${1:` + snippetEscape(display) + `}]`, true
	}
	return `#link("` + ref + `")[` + display + `]`, false
}

// bestLinkWindow finds the trailing window of line (beginning at a word
// boundary) that best fuzzy-matches the start of title. It returns the window's
// byte length and its similarity score in [0,1]. n is 0 when no window of at
// least minPhraseMatch characters exists. Only windows up to roughly the title
// length are considered, since longer ones can never score well.
func bestLinkWindow(line, title string) (n int, score float64) {
	titleLower := []rune(strings.ToLower(title))
	lo := len(line) - (len(title) + 6)
	if lo < 0 {
		lo = 0
	}
	for s := lo; s < len(line); s++ {
		if utf8.RuneCountInString(line[s:]) < minPhraseMatch {
			break // every later window is shorter still
		}
		if !isWordByte(line[s]) || (s > 0 && isWordByte(line[s-1])) {
			continue // candidate must start at a word boundary
		}
		sc := fuzzyPrefixScore([]rune(strings.ToLower(line[s:])), titleLower)
		if sc > score {
			score, n = sc, len(line)-s
		}
	}
	return n, score
}

// fuzzyPrefixScore scores how closely cand matches the beginning of title: it is
// 1 minus the edit distance from cand to the nearest prefix of title, normalized
// by cand's length. A clean (possibly partial) prefix scores 1.0; typos lower
// it. Both inputs must already be lower-cased.
func fuzzyPrefixScore(cand, title []rune) float64 {
	if len(cand) == 0 {
		return 0
	}
	d := minPrefixEditDistance(cand, title)
	return 1 - float64(d)/float64(len(cand))
}

// minPrefixEditDistance returns the smallest Levenshtein distance between a and
// any prefix of b (i.e. b is allowed to extend past the match for free).
func minPrefixEditDistance(a, b []rune) int {
	prev := make([]int, len(b)+1) // distances for a[:0] == j insertions
	for j := range prev {
		prev[j] = j
	}
	curr := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	best := prev[0]
	for _, d := range prev {
		if d < best {
			best = d
		}
	}
	return best
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
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
