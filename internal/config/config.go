package config

import (
	"encoding/json"
	"io"
)

type Config struct {
	Query       string `json:"query"        required:"true"`
	SelectRegex string `json:"select_regex" required:"true"`
	Root        string `json:"root"         required:"false"` // only for dump!
}

// LoadFromJSON reads JSON from r into a Config.
func LoadFromJSON(r io.Reader) (Config, error) {
	var cfg Config
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
