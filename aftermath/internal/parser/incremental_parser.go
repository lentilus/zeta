package parser

import (
	"aftermath/bindings"
	"context"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

// IncrementalParser implements the Parser interface
type IncrementalParser struct {
	parser     *sitter.Parser
	lang       *sitter.Language
	tree       *sitter.Tree
	query      *sitter.Query
	content    []byte
	references []Reference
	mu         sync.RWMutex
}

func NewIncrementalParser() (*IncrementalParser, error) {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)

	query, err := sitter.NewQuery(refQuery, lang)
	if err != nil {
		parser.Close()
		return nil, err
	}

	return &IncrementalParser{
		parser: parser,
		lang:   lang,
		query:  query,
	}, nil
}

// Parse implements Parser.Parse
func (ip *IncrementalParser) Parse(ctx context.Context, content []byte) error {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	tree, err := ip.parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return err
	}

	if ip.tree != nil {
		ip.tree.Close()
	}

	ip.tree = tree
	ip.content = content
	ip.references = ip.parseReferences()
	return nil
}

// ApplyChanges implements Parser.ApplyChanges
func (ip *IncrementalParser) ApplyChanges(ctx context.Context, changes []Change) error {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	// Convert Changes to tree-sitter edits
	for _, change := range changes {
		startOffset := ip.positionToOffset(change.Range.Start)
		endOffset := ip.positionToOffset(change.Range.End)

		edit := sitter.EditInput{
			StartIndex:  startOffset,
			OldEndIndex: endOffset,
			NewEndIndex: startOffset + uint32(len(change.NewText)),
			StartPoint:  convertPosition(change.Range.Start),
			OldEndPoint: convertPosition(change.Range.End),
			NewEndPoint: calculateEndPoint(ip.content, change),
		}

		// Apply edit to content
		newContent := make([]byte, 0, len(ip.content)-int(endOffset-startOffset)+len(change.NewText))
		newContent = append(newContent, ip.content[:startOffset]...)
		newContent = append(newContent, []byte(change.NewText)...)
		newContent = append(newContent, ip.content[endOffset:]...)

		ip.tree.Edit(edit)
		ip.content = newContent
	}

	// Reparse with changes
	newTree, err := ip.parser.ParseCtx(ctx, ip.tree, ip.content)
	if err != nil {
		return err
	}

	if ip.tree != newTree {
		ip.tree.Close()
	}
	ip.tree = newTree
	ip.references = ip.parseReferences()
	return nil
}

// GetReferences implements Parser.GetReferences
func (ip *IncrementalParser) GetReferences() []Reference {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	refs := make([]Reference, len(ip.references))
	copy(refs, ip.references)
	return refs
}

// GetReferenceAt implements Parser.GetReferenceAt
func (ip *IncrementalParser) GetReferenceAt(pos Position) (Reference, bool) {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	point := convertPosition(pos)
	node := ip.tree.RootNode().NamedDescendantForPointRange(point, point)
	if node == nil || node.Type() != "ref" {
		return Reference{}, false
	}

	rawContent := node.Content(ip.content)
	nodeRange := node.Range()
	ref := Reference{
		Target: processReferenceTarget(rawContent),
		Range: Range{
			Start: Position{Line: nodeRange.StartPoint.Row, Character: nodeRange.StartPoint.Column},
			End:   Position{Line: nodeRange.EndPoint.Row, Character: nodeRange.EndPoint.Column},
		},
	}
	return ref, true
}

// Close implements Parser.Close
func (ip *IncrementalParser) Close() error {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	if ip.query != nil {
		ip.query.Close()
	}
	if ip.tree != nil {
		ip.tree.Close()
	}
	if ip.parser != nil {
		ip.parser.Close()
	}
	return nil
}

// Helper methods
func (ip *IncrementalParser) positionToOffset(pos Position) uint32 {
	var offset uint32
	var currentLine uint32

	for i := 0; i < len(ip.content) && currentLine < pos.Line; i++ {
		if ip.content[i] == '\n' {
			currentLine++
		}
		offset++
	}

	offset += pos.Character
	if offset > uint32(len(ip.content)) {
		offset = uint32(len(ip.content))
	}

	return offset
}

func (ip *IncrementalParser) parseReferences() []Reference {
	if ip.tree == nil || len(ip.content) == 0 {
		return nil
	}

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(ip.query, ip.tree.RootNode())

	var refs []Reference
	for {
		match, captureIndex, ok := cursor.NextCapture()
		if !ok {
			break
		}

		if len(match.Captures) <= int(captureIndex) {
			continue
		}

		capture := match.Captures[captureIndex]
		if capture.Node == nil {
			continue
		}

		rawContent := capture.Node.Content(ip.content)
		nodeRange := capture.Node.Range()
		ref := Reference{
			Target: processReferenceTarget(rawContent),
			Range: Range{
				Start: Position{
					Line:      nodeRange.StartPoint.Row,
					Character: nodeRange.StartPoint.Column,
				},
				End: Position{
					Line:      nodeRange.EndPoint.Row,
					Character: nodeRange.EndPoint.Column,
				},
			},
		}
		refs = append(refs, ref)
	}

	return refs
}
