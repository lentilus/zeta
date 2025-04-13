package cache

import (
	"errors"
)

type Path string

// note represents a cached note.
type note struct {
	missing bool
	Path    Path
}

// Link represents a connection between notes.
// TODO: Changing this to an LSP like Range makes outside use easier
type Link struct {
	Row uint
	Col uint
	Ref string
	Src Path
	Tgt Path
}

// Event describes an Update to a subscriber.
type Event struct {
	Operation string // CREATE, DELETE, UPDATE (Update only on missing Notes)
	Group     string // NOTE or LINK
	ID        string // unique id for Element
	Data      any    // (path, missing) or (src, tgt) or ()
}

// Errors for Cache to use
var (
	ErrInvalidLink  = errors.New("cache: invalid link, source mismatch")
	ErrNoteNotFound = errors.New("cache: note not found")
)

// Cache provides methods to manipulate a cache of notes and links.
type Cache interface {
	// Upsert inserts or updates a note with its associated links.
	Upsert(path Path, links []Link) error

	// UpsertShadow inserts or updates a shadowing note with its associated links.
	UpsertTmp(path Path, links []Link) error

	// Delete removes a note from the cache.
	Delete(path Path) error

	// DeleteShadow removes a shadowing note from the cache.
	DeleteTmp(path Path) error

	// Paths retrieves all paths of notes.
	Paths() ([]Path, error)

	// ForwardLinks returns the links originating from the note at the given path.
	ForwardLinks(path Path) ([]Link, error)

	// BackLinks returns the links pointing to the note at the given path.
	BackLinks(path Path) ([]Link, error)

	// Flush writes any in-memory changes to persistent storage.
	Flush() error

	// Subscribe allows clients to receive updates from the cache.
	// returns initial Graph as well
	Subscribe() error
}
