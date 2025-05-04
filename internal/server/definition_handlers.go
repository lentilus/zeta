package server

import (
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
					return protocol.Location{URI: target.URI}, nil
				}
				context.Notify(
					"window/showDocument",
					protocol.ShowDocumentParams{
						URI:      protocol.URI(target.URI),
						External: &protocol.False,
					},
				)
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
	counter := 0

	var symbols []protocol.SymbolInformation

	for _, note := range notes {
		path := string(note)
		if isSubsequence(query, path) {
			resolved, _ := resolver.Resolve(note)
			symbols = append(symbols, protocol.SymbolInformation{
				Name:     path,
				Kind:     protocol.SymbolKindFile,
				Location: protocol.Location{URI: resolved.URI},
			})
			counter += 1
			if counter == max_results {
				break
			}
		}
	}
	return symbols, nil
}

func isSubsequence(pattern, text string) bool {
	// convert pattern to runes so we compare full Unicode codepoints
	pr := []rune(pattern)
	if len(pr) == 0 {
		return true // empty pattern always matches
	}

	i := 0
	for _, r := range text {
		if r == pr[i] {
			i++
			if i == len(pr) {
				return true
			}
		}
	}
	return false
}
