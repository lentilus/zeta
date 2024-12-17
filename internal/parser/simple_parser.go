package parser

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
	"zeta/bindings"

	sitter "github.com/smacker/go-tree-sitter"
)

// OneTimeParser implements SimpleParser
type OneTimeParser struct {
	parserPool  sync.Pool
	lang        *sitter.Language
	activeCount atomic.Int32
	maxPoolSize int32
	config      Config
}

func NewOneTimeParser(config Config) (*OneTimeParser, error) {
	lang := sitter.NewLanguage(bindings.Language())
	p := &OneTimeParser{
		lang:        lang,
		maxPoolSize: 16,
		config:      config,
	}
	p.parserPool = sync.Pool{
		New: func() interface{} {
			// Only create new parser if we haven't reached the maximum
			if p.activeCount.Load() >= p.maxPoolSize {
				return nil
			}
			parser := sitter.NewParser()
			parser.SetLanguage(lang)
			p.activeCount.Add(1)
			return parser
		},
	}
	return p, nil
}

// ParseReferences implements SimpleParser.ParseReferences
func (p *OneTimeParser) ParseReferences(ctx context.Context, content []byte) ([]string, error) {
	// Get parser from pool
	var parser *sitter.Parser
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if p := p.parserPool.Get(); p != nil {
			parser = p.(*sitter.Parser)
			break
		}
		// If we couldn't get a parser, wait a bit and try again
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Small sleep to prevent tight loop
			// You might want to adjust this value based on your needs
			time.Sleep(time.Millisecond)
		}
	}
	defer p.parserPool.Put(parser)

	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	query, err := sitter.NewQuery([]byte(p.config.ReferenceQuery), p.lang)
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
			processedTarget := processReferenceTarget(p.config, rawContent)
			refs = append(refs, processedTarget)
		}
	}
}

// Close implements SimpleParser.Close
func (p *OneTimeParser) Close() error {
	// Clean up any remaining parsers in the pool
	for {
		parser := p.parserPool.Get()
		if parser == nil {
			break
		}
		parser.(*sitter.Parser).Close()
		p.activeCount.Add(-1)
	}
	return nil
}

// GetActiveParserCount returns the current number of active parsers
func (p *OneTimeParser) GetActiveParserCount() int32 {
	return p.activeCount.Load()
}
