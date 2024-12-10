package store

// Path represents the Path to a zettel relative
// to the root of the zettelkasten
type Path string

type Store interface {
	// Core Operations
	UpdateOne(path Path) error
	UpdateAll() error
	Recompute() error

	// Queries
	GetAll() ([]Path, error)
	GetParents(path Path) ([]Path, error)
	GetChildren(path Path) ([]Path, error)
}
