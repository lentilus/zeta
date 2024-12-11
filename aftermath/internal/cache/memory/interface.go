package memory

// Position represents a position in a document
type Position struct {
	Line      uint32
	Character uint32
}

// Range represents a range in a document
type Range struct {
	Start Position
	End   Position
}

// Change represents a change to be applied to a document
type Change struct {
	Range   Range
	NewText string
}

// Reference represents a link to another document
type Reference struct {
	Range  Range
	Target string
}

// Document represents an open document in memory
type Document interface {
	// Core operations
	GetContent() string
	ApplyChanges(changes []Change) error
	Close() error

	// Reference operations
	GetReferences() []Reference
	GetReferenceAt(pos Position) (Reference, bool)
}

// DocumentManager manages open documents in memory
type DocumentManager interface {
	// Document operations
	OpenDocument(path string, content string) (Document, error)
	GetDocument(path string) (Document, bool)
	CloseDocument(path string) error

	// Bulk operations
	CloseAll() error
}
