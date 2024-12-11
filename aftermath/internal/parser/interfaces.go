package parser

import (
	"context"
)

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

// Parser defines the interface for document parsing
type Parser interface {
	// Core parsing operations
	Parse(ctx context.Context, content []byte) error
	ApplyChanges(ctx context.Context, changes []Change) error

	// Reference operations
	GetReferences() []Reference
	GetReferenceAt(pos Position) (Reference, bool)

	Close() error
}

// SimpleParser defines a simplified interface for one-time parsing
type SimpleParser interface {
	ParseReferences(ctx context.Context, content []byte) ([]string, error)
	Close() error
}
