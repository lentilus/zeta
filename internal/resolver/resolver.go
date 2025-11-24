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
