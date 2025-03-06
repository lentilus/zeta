package cache

import "time"

// Core Types
type NotePath string

type Note struct {
    Path      NotePath
    TimeStamp time.Time
    OnDisk    bool
}

type Link struct {
    Reference string
    Source    string // TODO: use NotePath
    Target    string // TODO: use NotePath
    Row       int
    Col       int
}

// Identifiers
type NoteId struct {
    Path NotePath
}

type LinkId struct {
    Ref string
    Src string
    Row int
    Col int
}

// changeId is a sealed type
type changeId interface { isChangeId() }
func (NoteId) isChangeId() {}
func (LinkId) isChangeId() {}

// changeData is a sealed type
type changeData interface { isChangeData() }
func (Link) isChangeData() {}
func (Note) isChangeData() {}

// Operations & Objects
type ChangeOperation string
type ChangeObject string

const (
    Insert ChangeOperation = "Insert"
    Update ChangeOperation = "Update"
    Delete ChangeOperation = "Delete"

    NoteObject ChangeObject = "Note"
    LinkObject ChangeObject = "Link"
)

// Change Log Event
type ChangeLogEvent struct {
    Object    ChangeObject    // NoteObject or LinkObject
    Operation ChangeOperation // Insert, Update, Delete
    Id        changeId        // NoteID or LinkID
    Data      changeData      // Note or Link
}

// Cache Interface
type Cache interface {
    // TODO: use Note instead of NotePath
    UpsertNote(note NotePath, links []Link) error
    DeleteNote(note NotePath) error
    GetLastModified(note NotePath) (time.Time, error)

    GetExistingNotes() ([]NotePath, error)
    GetMissingNotes() ([]NotePath, error)
    GetAllNotes() ([]NotePath, error)

    GetForwardLinks(source NotePath) ([]Link, error)
    GetBackLinks(target NotePath) ([]Link, error)

    Subscribe() (<-chan ChangeLogEvent, error)
}
