package sqlite

import (
	"zeta/internal/cache/database"
	"zeta/internal/parser"
)

type Config struct {
	RootPath     string
	ParserConfig parser.Config
	DBConfig     database.Config
}
