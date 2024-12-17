package memory

import (
	"aftermath/internal/parser"
	"context"
	"fmt"
	"sync"
)

// ParserDocument implements the Document interface using a Parser
type ParserDocument struct {
	parser  parser.Parser
	content string
	mu      sync.RWMutex
}

// NewParserDocument creates a new ParserDocument with the given content
// If a specific parser is provided, it will be used; otherwise, a new IncrementalParser is created
func NewParserDocument(content string, p parser.Parser) (*ParserDocument, error) {
	// var err error

	// // Create new parser if none provided
	// if p == nil {
	// 	p, err = parser.NewIncrementalParser()
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to create parser: %w", err)
	// 	}
	// }

	// Parse initial content
	if err := p.Parse(context.Background(), []byte(content)); err != nil {
		p.Close() // Clean up on error
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}

	// Create document
	doc := &ParserDocument{
		parser:  p,
		content: content,
	}

	return doc, nil
}

func (d *ParserDocument) GetContent() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.content
}

func (d *ParserDocument) ApplyChanges(changes []Change) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Convert memory.Change to parser.Change
	parserChanges := make([]parser.Change, len(changes))
	for i, change := range changes {
		parserChanges[i] = parser.Change{
			Range: parser.Range{
				Start: parser.Position{
					Line:      change.Range.Start.Line,
					Character: change.Range.Start.Character,
				},
				End: parser.Position{
					Line:      change.Range.End.Line,
					Character: change.Range.End.Character,
				},
			},
			NewText: change.NewText,
		}
	}

	// Apply changes to parser
	if err := d.parser.ApplyChanges(context.Background(), parserChanges); err != nil {
		return err
	}

	// Update content based on changes
	for _, change := range changes {
		startOffset := d.positionToOffset(change.Range.Start)
		endOffset := d.positionToOffset(change.Range.End)

		// Ensure offsets are valid
		if startOffset > uint32(len(d.content)) {
			startOffset = uint32(len(d.content))
		}
		if endOffset > uint32(len(d.content)) {
			endOffset = uint32(len(d.content))
		}

		// Apply change to content
		newContent := make([]byte, 0, len(d.content)-int(endOffset-startOffset)+len(change.NewText))
		newContent = append(newContent, d.content[:startOffset]...)
		newContent = append(newContent, change.NewText...)
		newContent = append(newContent, d.content[endOffset:]...)
		d.content = string(newContent)
	}

	return nil
}

func (d *ParserDocument) GetReferences() []Reference {
	d.mu.RLock()
	defer d.mu.RUnlock()

	parserRefs := d.parser.GetReferences()
	refs := make([]Reference, len(parserRefs))

	for i, pRef := range parserRefs {
		refs[i] = Reference{
			Range: Range{
				Start: Position{
					Line:      pRef.Range.Start.Line,
					Character: pRef.Range.Start.Character,
				},
				End: Position{
					Line:      pRef.Range.End.Line,
					Character: pRef.Range.End.Character,
				},
			},
			Target: pRef.Target,
		}
	}

	return refs
}

func (d *ParserDocument) GetReferenceAt(pos Position) (Reference, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	parserPos := parser.Position{
		Line:      pos.Line,
		Character: pos.Character,
	}

	pRef, found := d.parser.GetReferenceAt(parserPos)
	if !found {
		return Reference{}, false
	}

	ref := Reference{
		Range: Range{
			Start: Position{
				Line:      pRef.Range.Start.Line,
				Character: pRef.Range.Start.Character,
			},
			End: Position{
				Line:      pRef.Range.End.Line,
				Character: pRef.Range.End.Character,
			},
		},
		Target: pRef.Target,
	}

	return ref, true
}

func (d *ParserDocument) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.parser != nil {
		if err := d.parser.Close(); err != nil {
			return fmt.Errorf("failed to close parser: %w", err)
		}
		d.parser = nil
	}

	return nil
}

func (d *ParserDocument) positionToOffset(pos Position) uint32 {
	var offset uint32
	var currentLine uint32

	for i := 0; i < len(d.content) && currentLine < pos.Line; i++ {
		if d.content[i] == '\n' {
			currentLine++
		}
		offset++
	}

	offset += pos.Character
	if offset > uint32(len(d.content)) {
		offset = uint32(len(d.content))
	}

	return offset
}
