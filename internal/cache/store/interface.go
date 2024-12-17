package store

type Store interface {
	// Core Operations
	UpdateOne(path string) error
	UpdateAll() error
	Recompute() error

	// Queries
	GetAll() ([]string, error)
	GetParents(path string) ([]string, error)
	GetChildren(path string) ([]string, error)
}
