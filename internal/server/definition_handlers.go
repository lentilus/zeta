package server

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"unicode/utf8"
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
	maxResults := 128
	query := params.Query

	notes := s.cache.GetPaths()

	titles := make([]string, 0, len(notes))
	uris := make(map[string]string, len(notes))

	for _, n := range notes {
		meta, _ := s.cache.GetMetaData(n)
		resolved, _ := resolver.Resolve(string(n))
		t := resolver.Title(n, meta)
		uris[t] = resolved.URI
		titles = append(titles, t)
	}

	k := 2 // tolerate up to 2 typos
	hits := filterByBitapFuzzyParallel(query, titles, k, maxResults)

	var symbols []protocol.SymbolInformation
	for _, h := range hits {
		uri := uris[h]
		symbols = append(symbols, protocol.SymbolInformation{
			Name:     h,
			Kind:     protocol.SymbolKindFile,
			Location: protocol.Location{URI: uri},
		})
	}
	return symbols, nil
}

// filterByBitapFuzzyParallel filters paths by approximate Bitap matching with k errors
func filterByBitapFuzzyParallel(pattern string, paths []string, k, maxHits int) []string {
	if utf8.RuneCountInString(pattern) == 0 {
		return nil
	}

	patternRunes := []rune(pattern)
	m := len(patternRunes)
	if m == 0 {
		return nil
	}
	if m > 63 {
		patternRunes = patternRunes[:63]
		m = 63
	}

	var masks [128]uint64
	for i, r := range patternRunes {
		if r < 128 {
			masks[r] |= 1 << uint(i)
		}
	}

	highest := uint64(1) << uint(m-1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan string, maxHits)
	var hitCount int32

	sem := make(chan struct{}, runtime.GOMAXPROCS(0))

	for _, s := range paths {
		if atomic.LoadInt32(&hitCount) >= int32(maxHits) || ctx.Err() != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(text string) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			if bitapFuzzyMatch(text, masks, highest, k) {
				count := atomic.AddInt32(&hitCount, 1)
				if count <= int32(maxHits) {
					results <- text
					if count == int32(maxHits) {
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

// bitapFuzzyMatch returns true if pattern appears in text with at most k errors
func bitapFuzzyMatch(text string, masks [128]uint64, highest uint64, k int) bool {
	r := make([]uint64, k+1)
	for d := 0; d <= k; d++ {
		r[d] = 0
	}

	for _, cr := range text {
		var charMask uint64
		if cr < 128 {
			charMask = masks[cr]
		} else {
			charMask = 0
		}

		// Update R[0]
		r0 := ((r[0] << 1) | 1) & charMask
		r[0] = r0

		// Update R[d] for 1..k errors
		prevRd1 := r0
		for d := 1; d <= k; d++ {
			// Substitution / match
			rx := ((r[d] << 1) | 1) & charMask
			xi := (r[d] << 1) | 1
			xd := prevRd1

			newRd := rx | xi | xd
			prevRd1 = r[d]
			r[d] = newRd
		}

		// If any R[d] has bit (m-1) set, match within d errors
		for d := 0; d <= k; d++ {
			if (r[d] & highest) != 0 {
				return true
			}
		}
	}
	return false
}
