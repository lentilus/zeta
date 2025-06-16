package server

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"zeta/internal/resolver"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentDefinition(
	context *glsp.Context,
	params *protocol.DefinitionParams,
) (any, error) {
	note, _ := resolver.Resolve(params.TextDocument.URI)

	refs, err := s.cache.GetForwardLinks(note.CachePath)
	if err != nil {
		return nil, err
	}
	doc, err := s.manager.GetDocument(note.URI)
	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		for _, r := range ref.Ranges {
			indexFrom, indexTo := r.IndexesIn(string(doc))
			index := params.TextDocumentPositionParams.Position.IndexIn(string(doc))

			if index >= indexFrom && index <= indexTo {
				target, _ := resolver.Resolve(ref.Target)
				if s.cache.NoteExists(target.RelativePath) {
					return protocol.Location{
						URI: target.URI,
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 0},
							End:   protocol.Position{Line: 0, Character: 0},
						},
					}, nil
				}
				context.Notify(
					"window/showDocument",
					protocol.ShowDocumentParams{
						URI:      protocol.URI(target.URI),
						External: &protocol.False,
					},
				)
				return nil, nil
			}
		}
	}
	return nil, nil
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	note, _ := resolver.Resolve(params.TextDocument.URI)

	refs, err := s.cache.GetBackLinks(note.CachePath)
	if err != nil {
		return nil, err
	}

	var locations []protocol.Location
	for _, ref := range refs {
		for _, r := range ref.Ranges {
			source, _ := resolver.Resolve(ref.Source)
			locations = append(locations, protocol.Location{URI: source.URI, Range: r})
		}
	}

	return locations, nil
}

func (s *Server) workspaceSymbol(
	context *glsp.Context,
	params *protocol.WorkspaceSymbolParams,
) ([]protocol.SymbolInformation, error) {
	max_results := 128
	query := params.Query

	notes := s.cache.GetPaths()

	titles := make([]string, 0, len(notes))
	uris := make(map[string]string, len(notes))

	for _, n := range notes {
		meta, _ := s.cache.GetMetaData(n)
		resolved, _ := resolver.Resolve(string(n))
		n := resolver.Title(n, meta)
		uris[n] = resolved.URI
		titles = append(titles, n)
	}

	hits := filterWithinDistanceParallel(query, titles, 3, max_results)

	var symbols []protocol.SymbolInformation

	for _, h := range hits {
		uri, _ := uris[h]
		symbols = append(symbols, protocol.SymbolInformation{
			Name:     h,
			Kind:     protocol.SymbolKindFile,
			Location: protocol.Location{URI: uri},
		})
	}
	return symbols, nil
}

func levenshteinDistance(a, b string) int {
	if len(a) < len(b) {
		a, b = b, a
	}
	lenA, lenB := len(a), len(b)

	prev := make([]int, lenB+1)
	for j := 0; j <= lenB; j++ {
		prev[j] = j
	}

	for i := 1; i <= lenA; i++ {
		cur := make([]int, lenB+1)
		cur[0] = i
		for j := 1; j <= lenB; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			deletion := prev[j] + 1
			insertion := cur[j-1] + 1
			substitution := prev[j-1] + cost
			cur[j] = min(deletion, insertion, substitution)
		}
		prev = cur
	}
	return prev[lenB]
}

func filterWithinDistanceParallel(target string, paths []string, k, maxHits int) []string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan string, maxHits)
	var hitCount int32

	sem := make(chan struct{}, runtime.GOMAXPROCS(0))

	for _, s := range paths {

		// Check for early termination
		if atomic.LoadInt32(&hitCount) >= int32(maxHits) || ctx.Err() != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(str string) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			distance := levenshteinDistance(target, str)
			if distance < k {
				// Increment and check hit count atomically
				count := atomic.AddInt32(&hitCount, 1)
				if count <= int32(maxHits) {
					results <- str
					if count == int32(maxHits) {
						// reached limit, cancel remaining
						cancel()
					}
				}
			}
		}(s)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var filtered []string
	for s := range results {
		filtered = append(filtered, s)
	}
	return filtered
}
