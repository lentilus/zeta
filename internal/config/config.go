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

	// DisplayTemplate / DisplaySubstitutions build the text inserted into a
	// link's display body on completion. This is kept separate from the title
	// so classifiers (e.g. a `<Tool>` taxon used to disambiguate notes in the
	// picker) can be dropped from the rendered link text.
	DisplayTemplate      string   `json:"display_template"`
	DisplaySubstitutions []string `json:"display_substitutions"`

	// SuggestLinksInText controls whether, while writing ordinary text, the
	// server suggests turning the phrase being typed into a link to an existing
	// note whose title it matches.
	SuggestLinksInText bool `json:"suggest_links_in_text"`

	// LinkSuggestThreshold is the minimum fuzzy-match closeness (0..1) between
	// the typed phrase and a note title required to surface a prose link
	// suggestion. A clean (possibly partial) prefix scores 1.0; typos lower the
	// score. Raise it to reduce suggestions, lower it to allow looser matches.
	LinkSuggestThreshold float64 `json:"link_suggest_threshold"`

	// CompletionInsertDisplay controls whether accepting a link completion also
	// fills the link's display body (e.g. `#link("id")[Title]`) when it is empty
	// or absent. Existing non-empty bodies are never overwritten.
	CompletionInsertDisplay bool `json:"completion_insert_display"`

	// NewNoteIDScheme determines how the filename of a note created from a link
	// is generated: "random" (opaque short id, the default), "timestamp", or
	// "slug" (derived from the title). Ignored when NewNoteCommand is set.
	NewNoteIDScheme string `json:"new_note_id_scheme"`

	// NewNoteTemplate is the content written to a newly created note. A single
	// %s is substituted with the note's title.
	NewNoteTemplate string `json:"new_note_template"`

	// NewNoteCommand, when non-empty, is an external command used to create a
	// note instead of NewNoteTemplate. The placeholders {title}, {path}, {id}
	// and {root} are substituted before execution; the command is responsible
	// for writing the file at {path}.
	NewNoteCommand string `json:"new_note_command"`
}

var defaultConfig = Config{
	Root: ".",
	Typst: parser.Format{
	    Query:              `(call item: (ident) @link (#eq? @link "link") (group (string) @target ))`,
	    SelectRegex:        `^"(.*)"$`,
	    DefaultExtension:   ".typ",
	    TitleTemplate:      "%s %s %s",
	    TitleSubstitutions: []string{"taxon", "title", "path"},
		// TODO: display template and display substitutions
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

	SuggestLinksInText:      true,
	LinkSuggestThreshold:    0.7,
	CompletionInsertDisplay: true,
	NewNoteIDScheme:         "random",
	NewNoteTemplate:         "= %s\n",
	NewNoteCommand:          "",
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
