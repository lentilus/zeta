// Package cache provides an in-memory graph of notes and links with
// support for live update events and persistence.
package cache

import (
	"context"
	"errors"

	lsp "github.com/tliron/glsp/protocol_3_16"
)

type Path = string

// Path represents a node in the cache graph.
type Note = struct {
	Path        Path
	Placeholder bool
}

// Link represents a directed edge between two notes.
// Range locates the link in the source document
type Link struct {
	Source Path
	Target Path
	Ranges []lsp.Range
}

type EventType int

const (
	CreateNote EventType = iota // A new Note was added.
	UpdateNote                  // An existing Note was modified.
	DeleteNote                  // A Note was removed entirely
	CreateLink                  // A new Link was added.
	DeleteLink                  // A Link was removed.
)

// LinkEvent carries only topology (no Range) for event subscribers.
// NOTE: Links shall not be deleted then created again if they only
// differ in their range.
type LinkEvent struct {
	Source Path `json:"source"`
	Target Path `json:"target"`
}

type NoteEvent = Note

// Event describes a single change in the cache.
type Event struct {
	Type EventType
	Note *NoteEvent // Populated for note events.
	Link *LinkEvent // Populated for link events.
}

// Predefined errors returned by cache operations.
var (
	ErrInvalidLink  = errors.New("cache: invalid link; source does not match")
	ErrNoteNotFound = errors.New("cache: note not found")
)

type Graph interface {
	// UpsertNote
	UpsertNote(note Path, forwardLinks []Link) error

	// DeleteNote deletes a note or marks is missing if it has backlinks.
	DeleteNote(path Path) error

	// GetPaths returns all cached note paths.
	GetPaths() []Path

	// GetForwardLinks returns outgoing links from the given note.
	GetForwardLinks(path Path) ([]Link, error)

	// GetBackLinks returns incoming links to the given note.
	GetBackLinks(path Path) ([]Link, error)

	IsPlaceholder(path Path) (bool, error)

	// Subscribe returns a channel of change events until ctx is canceled.
	// Callers must drain the channel to avoid blocking.
	Subscribe(ctx context.Context) (<-chan Event, error)
}
