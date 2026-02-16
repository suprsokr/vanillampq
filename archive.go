package vanillampq

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/suprsokr/go-mpq"
)

// Archive wraps go-mpq with vanilla-specific functionality
type Archive struct {
	mpq     *mpq.Archive
	path    string
	version VanillaVersion
}

// Open opens a vanilla WoW MPQ archive for reading
func Open(path string) (*Archive, error) {
	if err := ValidateArchiveName(filepath.Base(path)); err != nil {
		return nil, fmt.Errorf("invalid archive name: %w", err)
	}

	mpqArchive, err := mpq.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open MPQ: %w", err)
	}

	return &Archive{
		mpq:  mpqArchive,
		path: path,
	}, nil
}

// Create creates a new vanilla-compatible MPQ archive
func Create(path string, maxFiles int) (*Archive, error) {
	if err := ValidateArchiveName(filepath.Base(path)); err != nil {
		return nil, fmt.Errorf("invalid archive name: %w", err)
	}

	// Vanilla always uses V1 format
	mpqArchive, err := mpq.Create(path, maxFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to create MPQ: %w", err)
	}

	return &Archive{
		mpq:  mpqArchive,
		path: path,
	}, nil
}

// OpenForModify opens an existing archive for modification
func OpenForModify(path string) (*Archive, error) {
	if err := ValidateArchiveName(filepath.Base(path)); err != nil {
		return nil, fmt.Errorf("invalid archive name: %w", err)
	}

	mpqArchive, err := mpq.OpenForModify(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open for modify: %w", err)
	}

	return &Archive{
		mpq:  mpqArchive,
		path: path,
	}, nil
}

// Close closes the archive
func (a *Archive) Close() error {
	return a.mpq.Close()
}

// ListFiles returns all file paths in the archive
func (a *Archive) ListFiles() ([]string, error) {
	return a.mpq.ListFiles()
}

// HasFile checks if a file exists in the archive
func (a *Archive) HasFile(path string) bool {
	return a.mpq.HasFile(path)
}

// OpenFile opens a file within the archive for reading
func (a *Archive) OpenFile(path string) (io.ReadCloser, error) {
	return a.mpq.OpenFile(path)
}

// ReadFile reads an entire file from the archive
func (a *Archive) ReadFile(path string) ([]byte, error) {
	return a.mpq.ReadFile(path)
}

// AddFile adds a file to the archive from a local path
func (a *Archive) AddFile(localPath, mpqPath string) error {
	// Normalize path to use backslashes (vanilla convention)
	mpqPath = NormalizePath(mpqPath)
	return a.mpq.AddFile(localPath, mpqPath)
}

// AddFileFromReader adds a file to the archive from a reader
func (a *Archive) AddFileFromReader(reader io.Reader, mpqPath string, size int64) error {
	mpqPath = NormalizePath(mpqPath)
	return a.mpq.AddFileFromReader(reader, mpqPath, size)
}

// AddFileFromBytes adds a file to the archive from a byte slice
func (a *Archive) AddFileFromBytes(data []byte, mpqPath string) error {
	mpqPath = NormalizePath(mpqPath)
	return a.mpq.AddFileFromBytes(data, mpqPath)
}

// RemoveFile removes a file from the archive
func (a *Archive) RemoveFile(mpqPath string) error {
	mpqPath = NormalizePath(mpqPath)
	return a.mpq.RemoveFile(mpqPath)
}

// NormalizePath converts forward slashes to backslashes (vanilla convention)
func NormalizePath(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}

// GetDBCFiles returns all .dbc files in the archive
func (a *Archive) GetDBCFiles() ([]string, error) {
	files, err := a.ListFiles()
	if err != nil {
		return nil, err
	}

	var dbcFiles []string
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file), ".dbc") {
			dbcFiles = append(dbcFiles, file)
		}
	}
	return dbcFiles, nil
}

// Path returns the archive file path
func (a *Archive) Path() string {
	return a.path
}
