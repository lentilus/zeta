package cache

import (
	"errors"
	"time"

	lsp "github.com/tliron/glsp/protocol_3_16"
)

type Path string

// note represents a cached note.
type note struct {
	missing   bool
	Path      Path
	Timestamp time.Time
}

// Link represents a connection between notes.
type Link struct {
	Range lsp.Range
	Src   Path
	Tgt   Path
}

type LinkData struct {
	SourceID int
	TargetID int
}

type NoteData struct {
	ID      int
	Path    string
	Missing bool
}

// Event describes an Update to a subscriber.
type Event struct {
	Operation string // "create/update/deleteNote" or "create/deleteLink"
	Link      LinkData
	Note      NoteData
}

// Errors for Cache to use
var (
	ErrInvalidLink  = errors.New("cache: invalid link, source mismatch")
	ErrNoteNotFound = errors.New("cache: note not found")
)

// Cache provides methods to manipulate a cache of notes and links.
type Cache interface {
	// Upsert inserts or updates a note with its associated links.
	Upsert(path Path, links []Link, time time.Time) error

	// UpsertTmp inserts or updates a shadowing note with its associated links.
	UpsertTmp(path Path, links []Link) error

	// Delete removes a note from the cache.
	Delete(path Path) error

	// DeleteTmp removes a shadowing note from the cache.
	DeleteTmp(path Path) error

	// Paths retrieves all paths of notes.
	Paths() ([]Path, error)

	// ForwardLinks returns the links originating from the note at the given path.
	ForwardLinks(path Path) ([]Link, error)

	// BackLinks returns the links pointing to the note at the given path.
	BackLinks(path Path) ([]Link, error)

	// Timestamp returns the timestamp of a persistent note or errors.
	Timestamp(path Path) (time.Time, error)

	// Subscribe allows clients to receive updates from the cache.
	Subscribe() (<-chan Event, func(), error)

	Dump() ([]byte, error)
}
