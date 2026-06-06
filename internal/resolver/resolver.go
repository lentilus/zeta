package resolver

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"zeta/internal/cache"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Note struct {
	URI          protocol.DocumentUri
	AbsolutePath string
	RelativePath string
	CachePath    cache.Path
	Format		 string // e.g. "typst" or "markdown"
}

var (
	configured bool = false
	rootConfig string
	extensionsConfig map[string]string
)

func Configure(root string, extensions map[string]string) error {
	if configured {
		panic("Resolver already configured.")
	}
	configured = true
	rootConfig = root
	extensionsConfig = extensions

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
		return resolveAbsolute(filepath.Join(rootConfig, v))
	default:
		return Note{}, fmt.Errorf("Invalid base type.")
	}
}

func IngoreDir(absolutepath string) bool {
	rel, err := filepath.Rel(rootConfig, absolutepath)
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

	rel, err := filepath.Rel(rootConfig, cleaned)
	if err != nil {
		log.Printf("resolveAbsolute errored with %v", err)
		return Note{}, err
	}

	ext := filepath.Ext(cleaned)
	format, ok := extensionsConfig[ext]
	if !ok{
		return Note{}, fmt.Errorf("Unrecognized extension %s", ext)
	}

	cachePath := cache.Path(rel)

	return Note{
		URI:          uri,
		AbsolutePath: cleaned,
		RelativePath: rel,
		CachePath:    cachePath,
		Format: format,
	}, nil
}

//
// TODO: this will depend on the format of the source note, so most of the below should probabably move into parser
//

// Reference returns the string to write inside a link so that it resolves back
// to the note at relativePath. The default extension is stripped (it is
// re-appended during resolution), yielding clean targets like "sub/d654bf".
func Reference(relativePath string, soureFormat string) string {
	ext := filepath.Ext(relativePath)
	if ext == defaultExtension {
		return strings.TrimSuffix(relativePath, ext)
	}
	return relativePath
}

// DefinitionNode returns the captured node that best represents a note's
// "definition": the earliest-positioned capture among the configured title
// substitutions (typically the heading/title). It returns nil if the document
// contains none of them, in which case callers fall back to the top of the
// file.
func DefinitionNode(namedNodes map[string][]*sitter.Node) *sitter.Node {
	var best *sitter.Node
	for _, key := range titleSubstitutions {
		nodes := namedNodes[key]
		if len(nodes) == 0 {
			continue
		}
		n := nodes[0]
		if best == nil || startsBefore(n, best) {
			best = n
		}
	}
	return best
}

// startsBefore reports whether node a starts earlier in the document than b.
func startsBefore(a, b *sitter.Node) bool {
	ap, bp := (*a).StartPoint(), (*b).StartPoint()
	if ap.Row != bp.Row {
		return ap.Row < bp.Row
	}
	return ap.Column < bp.Column
}
