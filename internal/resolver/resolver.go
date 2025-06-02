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
	configured         bool = false
	root               string
	selectRegex        *regexp.Regexp
	fileExtenstions    []string
	defaultExtension   string
	titleTemplate      string
	titleSubstitutions []string
)

func Configure(
	configRoot string,
	configSelectRegex string,
	configFileExtensions []string,
	configDefaultExtension string,
	configTitleTemplate string,
	configTitleSubstitutions []string,
) error {
	if configured {
		panic("Resolver already configured.")
	}

	root = configRoot
	fileExtenstions = configFileExtensions
	defaultExtension = configDefaultExtension
	titleTemplate = configTitleTemplate
	titleSubstitutions = configTitleSubstitutions

	var err error
	selectRegex, err = regexp.Compile(configSelectRegex)
	if err != nil {
		return err
	}
	return nil
}

func Title(path string, metadata map[string]string) string {
	if len(metadata) == 0 {
		return path
	}
	var args []any

	for _, s := range titleSubstitutions {
		v, ok := metadata[string(s)]
		if ok {
			args = append(args, v)
		} else {
			args = append(args, "")
		}
	}

	title := fmt.Sprintf(titleTemplate, args...)
	return title
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

	found := false
	ext := filepath.Ext(cleaned)
	for _, e := range fileExtenstions {
		if e == ext {
			found = true
		}
	}

	if !found {
		return Note{}, fmt.Errorf("No valid file extension.")
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

	// Add default extension if none is specified.
	if filepath.Ext(reference) == "" {
		reference += defaultExtension
	}

	// Check if path should be relative to note.
	if reference[0] == []byte(`.`)[0] {
		base := filepath.Dir(source.AbsolutePath)
		joined := filepath.Join(base, reference)
		return Resolve(joined)
	}

	return Resolve(reference)
}

func ExtractLinksAndMeta(
	note Note,
	namedNodes map[string][]*sitter.Node,
	document []byte,
) ([]cache.Link, map[string]string) {
	nodes := namedNodes["target"]
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

	meta := make(map[string]string)
	for k, v := range namedNodes {
		if len(v) > 0 {
			meta[k] = v[0].Content(document)
		}
	}

	return links, meta
}
