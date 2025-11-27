package config

import (
	"encoding/json"
	"fmt"
	"io"
	"zeta/internal/parser"
)

type Config struct {
	Root     string `json:"root"` // only for dump!
	Typst    parser.Format `json:"typst"`
	Markdown parser.Format `json:"markdown"`
	Extensions map[string]string `json:"extensions"`
}

var defaultConfig = Config{
	Root: ".",
	Typst: parser.Format{
	    Query:              `(call item: (ident) @link (#eq? @link "link") (group (string) @target ))`,
	    SelectRegex:        `^"(.*)"$`,
	    DefaultExtension:   ".typ",
	    TitleTemplate:      "%s %s %s",
	    TitleSubstitutions: []string{"taxon", "title", "path"},
	},
	Markdown: parser.Format{
		Query:              `(link text: (label) @label destination: (destination) @target )`,
	    SelectRegex:        `^(.*)$`,
	    DefaultExtension:   ".md",
	    TitleTemplate:      "%s %s %s",
	    TitleSubstitutions: []string{"taxon", "title", "path"},
	},
	Extensions: map[string]string {
		".typ" : "typst",
		".md" : "markdown",
	},
}

func Load(v any) (Config, error) {
	cfg := defaultConfig

	data, err := json.Marshal(v)
	if err != nil {
		return Config{}, fmt.Errorf("failed to marshal source: %w", err)
	}

	// only fields present in src will overwrite.
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal into Config: %w", err)
	}

	return cfg, nil
}

// LoadFromJSON reads JSON from r into a Config.
func LoadFromJSON(r io.Reader) (Config, error) {
	cfg := defaultConfig

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
