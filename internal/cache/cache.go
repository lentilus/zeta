package cache

import "time"

type Note string

type Link struct {
    Reference string
    Source string
    Target string
    Row int
    Col int
}

type ChangeLogEvent struct{}

type Cache interface {
    UpsertNote(note Note, links []Link) error
    DeleteNote(note Note) error
    GetLastModified(note Note) (time.Time, error)

    GetExistingNotes() ([]Note, error)
    GetMissingNotes() ([]Note, error)
    GetAllNotes() ([]Note, error)

    GetForwardLinks(source Note) ([]Link, error)
    GetBackLinks(target Note) ([]Link, error)

    Subscribe() (<-chan ChangeLogEvent, error)
}
