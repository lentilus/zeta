package parser

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"zeta/internal/cache"
	"zeta/internal/resolver"
	"zeta/internal/sitteradapter"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (p *Parser) genTitle(path string, metadata map[string]string) string {
	if len(metadata) == 0 {
		return path
	}
	var args []any

	for _, s := range p.format.TitleSubstitutions {
		v, ok := metadata[string(s)]
		if ok {
			args = append(args, v)
		} else {
			args = append(args, "")
		}
	}

	title := fmt.Sprintf(p.format.TitleTemplate, args...)
	title = strings.TrimSpace(title)
	return title
}

func (p *Parser)resolveReference(source resolver.Note, reference string) (resolver.Note, error) {
	if len(reference) == 0 {
		return resolver.Note{}, fmt.Errorf("Invalid path.")
	}

	reg, ok := regexes[p.format.SelectRegex]
	if ! ok {
		reg, _ = regexp.Compile(p.format.SelectRegex)
		regexes[p.format.SelectRegex] = reg
	}

	matches := reg.FindSubmatch([]byte(reference))
	if len(matches) < 2 {
		return resolver.Note{}, fmt.Errorf("Invalid reference")
	}
	match := matches[1]
	if match == nil {
		return resolver.Note{}, fmt.Errorf("Invalid reference")
	}

	reference = string(match)

	if reference == "" {
		return resolver.Note{}, fmt.Errorf("Empty reference.")
	}

	if strings.HasSuffix(reference, "/") {
		return resolver.Note{}, fmt.Errorf("Cannot reference directories.")
	}

	// Add default extension if none is specified.
	if filepath.Ext(reference) == "" {
		reference += p.format.DefaultExtension
	}

	// Check if path should be relative to note.
	if reference[0] == []byte(`.`)[0] {
		base := filepath.Dir(source.AbsolutePath)
		joined := filepath.Join(base, reference)
		return resolver.Resolve(joined)
	}

	return resolver.Resolve(reference)
}

func (p *Parser)ExtractLinksAndMeta(
	note resolver.Note,
	document []byte,
) ([]cache.Link, map[string]string) {
	namedNodes, err := p.query([]byte(p.format.Query), document)
	if err != nil {
		panic(err)
	}
	nodes := namedNodes["target"]
	// Map to group ranges by target path, preserving insertion order
	rangesMap := make(map[string][]protocol.Range)
	order := make([]string, 0, len(nodes))

	for _, n := range nodes {
		reference := (*n).Content(document)

		target, err := p.resolveReference(note, reference)
		if err != nil {
			continue
		}

		tgtPath := target.CachePath
		// Compute the range for this reference
		r := protocol.Range{
			Start: sitteradapter.TSPointToLSPPosition((*n).StartPoint(), string(document)),
			End:   sitteradapter.TSPointToLSPPosition((*n).EndPoint(), string(document)),
		}

		// Initialize entry and record order if first time seeing this target
		if _, exists := rangesMap[tgtPath]; !exists {
			order = append(order, tgtPath)
		}
		rangesMap[tgtPath] = append(rangesMap[tgtPath], r)
	}

	// Build slice of links grouped by target
	links := make([]cache.Link, 0, len(rangesMap))
	for _, tgtPath := range order {
		links = append(links, cache.Link{
			Source: note.CachePath,
			Target: tgtPath,
			Ranges: rangesMap[tgtPath],
		})
	}

	meta := make(map[string]string)
	for k, v := range namedNodes {
		if len(v) > 0 {
			meta[k] = v[0].Content(document)
		}
	}

	// NOTE: this would be nicer if it were not hardcoded
	meta["DISPLAY_TITLE"] = p.genTitle(note.CachePath, meta)

	return links, meta
}


