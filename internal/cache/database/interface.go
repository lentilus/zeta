package database

type FileRecord struct {
	Path         string
	LastModified int64
}

type LinkRecord struct {
	SourcePath string
	TargetPath string
}

type Database interface {
	// Transaction handling
	WithTx(fn func(tx Transaction) error) error

	// File operations
	GetFile(path string) (*FileRecord, error)
	GetAllFiles() ([]FileRecord, error)
	UpsertFile(file *FileRecord) error
	DeleteFile(path string) error

	// Link operations
	GetLinks(sourcePath string) ([]LinkRecord, error)
	GetBacklinks(targetPath string) ([]LinkRecord, error)
	UpsertLinks(sourcePath string, targetPaths []string) error
	DeleteLinks(sourcePath string) error

	// Maintenance
	Clear() error
	Close() error
}

type Transaction interface {
	UpsertFile(file *FileRecord) error
	UpsertLinks(sourcePath string, targetPaths []string) error
}
