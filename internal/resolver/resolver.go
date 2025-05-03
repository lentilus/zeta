package resolver

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"zeta/internal/cache"
	"zeta/internal/sitteradapter"

	sitter "github.com/smacker/go-tree-sitter"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Note struct {
	URI          protocol.DocumentUri
	AbsolutePath string
	RelativePath string
	CachePath    cache.Path
}

var (
	configured  bool = false
	root        string
	selectRegex *regexp.Regexp
)

func Configure(configRoot string, configSelectRegex string) error {
	if configured {
		panic("Resolver already configured.")
	}
	root = configRoot
	var err error
	selectRegex, err = regexp.Compile(configSelectRegex)
	if err != nil {
		return err
	}
	return nil
}

func Resolve(base any) (Note, error) {
	switch v := base.(type) {
	case string:
		url, err := url.Parse(v)
		if err != nil {
			return Note{}, err
		}
		path := url.Path
		if filepath.IsAbs(path) {
			return resolveAbsolute(path)
		}
		return resolveAbsolute(filepath.Join(root, v))
	default:
		return Note{}, fmt.Errorf("Invalid base type.")
	}
}

func IngoreDir(absolutepath string) bool {
	rel, err := filepath.Rel(root, absolutepath)
	if err != nil {
		return true
	}
	clean := filepath.Clean(rel)
	if clean == "." {
		return false
	}
	return strings.HasPrefix(clean, ".")
}

func resolveAbsolute(absolutepath string) (Note, error) {
	cleaned := filepath.Clean(absolutepath)
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(cleaned),
	}
	uri := protocol.DocumentUri(u.String())

	rel, err := filepath.Rel(root, cleaned)
	if err != nil {
		log.Printf("resolveAbsolute errored with %v", err)
		return Note{}, err
	}

	if filepath.Ext(cleaned) != ".typ" {
		return Note{}, fmt.Errorf("Not a typst file.")
	}

	cachePath := cache.Path(rel)

	return Note{
		URI:          uri,
		AbsolutePath: cleaned,
		RelativePath: rel,
		CachePath:    cachePath,
	}, nil
}

func ResolveReference(source Note, reference string) (Note, error) {
	if len(reference) == 0 {
		return Note{}, fmt.Errorf("Invalid path.")
	}

	matches := selectRegex.FindSubmatch([]byte(reference))
	if len(matches) < 2 {
		return Note{}, fmt.Errorf("Invalid reference")
	}
	match := matches[1]
	if match == nil {
		return Note{}, fmt.Errorf("Invalid reference")
	}

	reference = string(match)

	if reference == "" {
		return Note{}, fmt.Errorf("Empty reference.")
	}

	if strings.HasSuffix(reference, "/") {
		return Note{}, fmt.Errorf("Cannot reference directories.")
	}

	// Add `.typ` extension if none is specified.
	if filepath.Ext(reference) == "" {
		reference += ".typ"
	}

	// Check if path should be relative to note.
	if reference[0] == []byte(`.`)[0] {
		base := filepath.Dir(source.AbsolutePath)
		joined := filepath.Join(base, reference)
		return Resolve(joined)
	}

	return Resolve(reference)
}

func ExtractLinks(note Note, nodes []*sitter.Node, document []byte) []cache.Link {
	// Map to group ranges by target path, preserving insertion order
	rangesMap := make(map[string][]protocol.Range)
	order := make([]string, 0, len(nodes))

	for _, n := range nodes {
		reference := (*n).Content(document)

		target, err := ResolveReference(note, reference)
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

	return links
}
