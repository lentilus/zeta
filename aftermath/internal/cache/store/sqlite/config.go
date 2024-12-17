package sqlite

import (
	"aftermath/internal/parser"
)

type Config struct {
	DBPath       string
	BibPath      string
	RootPath     string
	ParserConfig parser.Config
}
