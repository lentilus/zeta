package server

import (
	"os"
	"strings"
	"zeta/internal/resolver"
	"zeta/internal/sitteradapter"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentDefinition(
	context *glsp.Context,
	params *protocol.DefinitionParams,
) (any, error) {
	note, err := resolver.Resolve(params.TextDocument.URI)
	if err != nil {
		return nil, nil
	}

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
				target, err := resolver.Resolve(ref.Target)
				if err != nil {
					return nil, nil
				}
				// Per the LSP spec the result is `Location | Location[] |
				// LocationLink[] | null`. We always return a Location pointing
				// at the target note: the editor opens it whether or not the
				// file already exists (placeholder notes are created on save).
				// Returning the result directly is the idiomatic answer to a
				// definition request, rather than side-effecting via a separate
				// `window/showDocument` request from inside this handler.
				return protocol.Location{
					URI:   target.URI,
					Range: s.definitionRange(target),
				}, nil
			}
		}
	}
	return nil, nil
}

// definitionRange locates the position of a note's title within the target
// document so goto-definition lands on the heading rather than the top of the
// file. It reads the open buffer if available, otherwise the file on disk, and
// falls back to the start of the file (0:0) for placeholder notes or when no
// title capture is present.
func (s *Server) definitionRange(target resolver.Note) protocol.Range {
	zero := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}

	// Prefer the in-memory document (unsaved edits); fall back to disk.
	content, err := s.manager.GetDocument(target.URI)
	if err != nil {
		content, err = os.ReadFile(target.AbsolutePath)
		if err != nil {
			// File does not exist yet (placeholder) or is unreadable.
			return zero
		}
	}

	nodes, err := s.parsers.ParseAndQuery(content, []byte(s.config.Query))
	if err != nil {
		return zero
	}
	node := resolver.DefinitionNode(nodes)
	if node == nil {
		return zero
	}

	pos := sitteradapter.TSPointToLSPPosition((*node).StartPoint(), string(content))
	return protocol.Range{Start: pos, End: pos}
}

func (s *Server) textDocumentReferences(
	context *glsp.Context,
	params *protocol.ReferenceParams,
) ([]protocol.Location, error) {
	note, err := resolver.Resolve(params.TextDocument.URI)
	if err != nil {
		return nil, nil
	}

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

		meta, _ := s.cache.GetMetaData(note)
		name := meta["DISPLAY_TITLE"]
		if isSubsequence(query, name) {
			resolved, _ := resolver.Resolve(note)
			symbols = append(symbols, protocol.SymbolInformation{
				Name:     name,
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
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)
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
