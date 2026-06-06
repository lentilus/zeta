package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"zeta/internal/resolver"
)

// newNoteID generates an identifier for a new note according to the configured
// scheme. The returned id is the bare filename stem (no extension).
func (s *Server) newNoteID(title string) string {
	switch s.config.NewNoteIDScheme {
	case "slug":
		return slugify(title)
	case "timestamp":
		return time.Now().Format("20060102T150405")
	default: // "random"
		return randomID()
	}
}

func randomID() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a timestamp-based id; collisions are handled by the caller.
		return time.Now().Format("20060102T150405.000")
	}
	return hex.EncodeToString(b)
}

var slugNonWord = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugNonWord.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return randomID()
	}
	return s
}

// generateNewNote allocates a unique, not-yet-existing note for the given title
// and returns the reference to insert in a link plus the resolved note.
func (s *Server) generateNewNote(title string) (ref string, note resolver.Note, err error) {
	for i := 0; i <= 64; i++ {
		id := s.newNoteID(title)
		if i > 0 {
			id = fmt.Sprintf("%s-%d", id, i)
		}
		candidate := id + s.config.DefaultExtension
		n, e := resolver.Resolve(candidate)
		if e != nil {
			continue
		}
		if _, statErr := os.Stat(n.AbsolutePath); statErr == nil {
			continue // file already exists; try another id
		}
		return resolver.Reference(n.CachePath), n, nil
	}
	return "", resolver.Note{}, fmt.Errorf("could not allocate a unique note id for %q", title)
}

// createNote materializes a note on disk. args is [relativePath, title]. It is
// invoked both by the createNote command (completion item / code action) and is
// a no-op if the file already exists.
func (s *Server) createNote(args []any) error {
	if len(args) < 2 {
		return fmt.Errorf("createNote: expected [path, title] arguments")
	}
	relPath, ok := args[0].(string)
	if !ok {
		return fmt.Errorf("createNote: path argument must be a string")
	}
	title, _ := args[1].(string)

	note, err := resolver.Resolve(relPath)
	if err != nil {
		return fmt.Errorf("createNote: %w", err)
	}
	if _, err := os.Stat(note.AbsolutePath); err == nil {
		return nil // already exists, nothing to do
	}

	if s.config.NewNoteCommand != "" {
		if err := s.runNewNoteCommand(note, title); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(note.AbsolutePath), 0o755); err != nil {
			return err
		}
		content := fmt.Sprintf(s.config.NewNoteTemplate, title)
		if err := os.WriteFile(note.AbsolutePath, []byte(content), 0o644); err != nil {
			return err
		}
	}

	s.indexNote(note)
	return nil
}

// runNewNoteCommand delegates note creation to the configured external command,
// substituting the {title}, {path}, {id} and {root} placeholders.
func (s *Server) runNewNoteCommand(note resolver.Note, title string) error {
	id := strings.TrimSuffix(filepath.Base(note.AbsolutePath), filepath.Ext(note.AbsolutePath))
	repl := strings.NewReplacer(
		"{title}", title,
		"{path}", note.AbsolutePath,
		"{id}", id,
		"{root}", resolver.Root(),
	)
	cmdStr := repl.Replace(s.config.NewNoteCommand)
	cmd := exec.Command("sh", "-c", cmdStr)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("new_note_command failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// indexNote parses a freshly created note and commits it to the cache so its
// placeholder is promoted to a real note (turning dangling-link warnings into
// resolved titles) without waiting for the file to be opened.
func (s *Server) indexNote(note resolver.Note) {
	content, err := os.ReadFile(note.AbsolutePath)
	if err != nil {
		return
	}
	nodes, err := s.parsers.ParseAndQuery(content, []byte(s.config.Query))
	if err != nil {
		return
	}
	links, meta := resolver.ExtractLinksAndMeta(note, nodes, content)
	_ = s.cache.SaveNote(note.CachePath, links, meta, time.Now())
}
