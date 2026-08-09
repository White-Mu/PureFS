package storage

import "io"

type FileInfo struct {
	Path     string
	Size     int64
	IsDir    bool
	SHA256   string
}

type Storage interface {
	// Open opens a file for reading.
	Open(path string) (io.ReadCloser, error)

	// Create creates or truncates a file and returns a writer.
	Create(path string) (io.WriteCloser, error)

	// Delete removes a file.
	Delete(path string) error

	// Stat returns info about a file.
	Stat(path string) (*FileInfo, error)

	// List returns all entries in a directory.
	List(dir string) ([]*FileInfo, error)

	// Mkdir creates a directory and all parents.
	Mkdir(path string) error

	// Rename moves a file/directory.
	Rename(oldPath, newPath string) error

	// Copy copies a file.
	Copy(srcPath, dstPath string) error

	// Exists checks if a path exists.
	Exists(path string) (bool, error)

	// RealPath returns the actual filesystem path for the given logical path.
	RealPath(logicalPath string) string
}
