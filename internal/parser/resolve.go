package parser

import (
	"fmt"
	"path/filepath"
)

// files starting with . are interpreted relative to file
// otherwise interpreted relative to root
func Resolve(source string, reference string) (string, error) {
	if len(reference) == 0 {
		return "", fmt.Errorf("Invalid path.")
	}

	// Add `.typ` extension if none is specified.
	if filepath.Ext(reference) == "" {
		reference += ".typ"
	}

	// Check if path should be relative to note.
	if reference[0] == []byte(`.`)[0] {
		base := filepath.Dir(source)
		joined := filepath.Join(base, reference)
		return filepath.Clean(joined), nil
	}

	return filepath.Clean(reference), nil
}
