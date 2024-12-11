package parser

import (
	"aftermath/bindings"
	"context"

	sitter "github.com/smacker/go-tree-sitter"
)

// OneTimeParser implements SimpleParser
type OneTimeParser struct {
	parser *sitter.Parser
	lang   *sitter.Language
}

func NewOneTimeParser() (*OneTimeParser, error) {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(bindings.Language())
	parser.SetLanguage(lang)
	return &OneTimeParser{parser: parser, lang: lang}, nil
}

// ParseReferences implements SimpleParser.ParseReferences
func (p *OneTimeParser) ParseReferences(ctx context.Context, content []byte) ([]string, error) {
	tree, err := p.parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	query, err := sitter.NewQuery(refQuery, p.lang)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, tree.RootNode())

	var refs []string
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		default:
			match, captureIndex, ok := cursor.NextCapture()
			if !ok {
				return refs, nil
			}

			if len(match.Captures) <= int(captureIndex) {
				continue
			}

			capture := match.Captures[captureIndex]
			if capture.Node == nil {
				continue
			}

			rawContent := capture.Node.Content(content)
			processedTarget := processReferenceTarget(rawContent)
			refs = append(refs, processedTarget)
		}
	}
}

// Close implements SimpleParser.Close
func (p *OneTimeParser) Close() error {
	if p.parser != nil {
		p.parser.Close()
	}
	return nil
}
