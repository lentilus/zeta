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
	log.Printf("Resolve was called with %v", base)
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
	case cache.Path:
		return resolveAbsolute(filepath.Join(root, string(v)))
	default:
		return Note{}, fmt.Errorf("Invalid base type.")
	}
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

	match := selectRegex.FindSubmatch([]byte(reference))[1]
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
	var links []cache.Link
	for _, n := range nodes {
		reference := (*n).Content(document)

		target, err := ResolveReference(note, reference)
		if err != nil {
			continue
		}

		l := cache.Link{
			Range: protocol.Range{
				Start: sitteradapter.TSPointToLSPPosition((*n).StartPoint(), string(document)),
				End:   sitteradapter.TSPointToLSPPosition((*n).EndPoint(), string(document)),
			},
			Src: note.CachePath,
			Tgt: target.CachePath,
		}

		links = append(links, l)
	}
	return links
}
