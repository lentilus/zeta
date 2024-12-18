package database

import "zeta/internal/bibliography"

type Config struct {
	Root               string
	CanonicalExtension string
	PathSeparator      string
	DBPath             string
	Bib                bibliography.Bibliography
}
