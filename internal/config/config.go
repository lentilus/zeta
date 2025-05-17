package config

import (
	"encoding/json"
	"fmt"
	"io"
)

type Config struct {
	Query            string   `json:"query"`
	SelectRegex      string   `json:"select_regex"`
	Root             string   `json:"root"` // only for dump!
	FileExtensions   []string `json:"file_extensions"`
	DefaultExtension string   `json:"default_extension"`
}

var defaultConfig = Config{
	Query:            `(call item: (ident) @link (#eq? @link "link") (group (string) @target ))`,
	SelectRegex:      `^"(.*)"$`,
	Root:             ".",
	FileExtensions:   []string{".typ"},
	DefaultExtension: ".typ",
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
