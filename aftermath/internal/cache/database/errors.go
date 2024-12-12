package database

import "fmt"

var (
	// ErrNotFound is returned when a requested record doesn't exist
	ErrNotFound = fmt.Errorf("record not found")

	// ErrInvalidTransaction is returned when a transaction operation fails
	ErrInvalidTransaction = fmt.Errorf("invalid transaction")

	// ErrConstraintViolation is returned when a database constraint is violated
	ErrConstraintViolation = fmt.Errorf("constraint violation")

	// ErrDatabaseClosed is returned when attempting to use a closed database
	ErrDatabaseClosed = fmt.Errorf("database is closed")
)
