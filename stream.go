package vanillampq

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileEntry represents a file being streamed from an archive
type FileEntry struct {
	Path   string        // Internal MPQ path
	Size   int64         // File size in bytes
	Reader io.ReadCloser // Reader for the file content
}

// StreamExtract streams files from an archive, yielding each file as a FileEntry
func StreamExtract(archivePath string) (<-chan FileEntry, <-chan error) {
	entries := make(chan FileEntry)
	errors := make(chan error, 1)

	go func() {
		defer close(entries)
		defer close(errors)

		archive, err := Open(archivePath)
		if err != nil {
			errors <- fmt.Errorf("failed to open archive: %w", err)
			return
		}
		defer archive.Close()

		files, err := archive.ListFiles()
		if err != nil {
			errors <- fmt.Errorf("failed to list files: %w", err)
			return
		}

		for _, path := range files {
			data, err := archive.ReadFile(path)
			if err != nil {
				errors <- fmt.Errorf("failed to read %s: %w", path, err)
				return
			}

			// Create a reader from the data
			reader := &bytesReadCloser{data: data}

			entries <- FileEntry{
				Path:   path,
				Size:   int64(len(data)),
				Reader: reader,
			}
		}
	}()

	return entries, errors
}

// StreamExtractDBCs streams only DBC files from an archive
func StreamExtractDBCs(archivePath string) (<-chan FileEntry, <-chan error) {
	entries := make(chan FileEntry)
	errors := make(chan error, 1)

	go func() {
		defer close(entries)
		defer close(errors)

		archive, err := Open(archivePath)
		if err != nil {
			errors <- fmt.Errorf("failed to open archive: %w", err)
			return
		}
		defer archive.Close()

		dbcFiles, err := archive.GetDBCFiles()
		if err != nil {
			errors <- fmt.Errorf("failed to get DBC files: %w", err)
			return
		}

		for _, path := range dbcFiles {
			data, err := archive.ReadFile(path)
			if err != nil {
				errors <- fmt.Errorf("failed to read %s: %w", path, err)
				return
			}

			reader := &bytesReadCloser{data: data}

			entries <- FileEntry{
				Path:   path,
				Size:   int64(len(data)),
				Reader: reader,
			}
		}
	}()

	return entries, errors
}

// ExtractAll extracts all files from an archive to a directory
func ExtractAll(archivePath, outputDir string) error {
	archive, err := Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer archive.Close()

	files, err := archive.ListFiles()
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	for _, mpqPath := range files {
		// Convert MPQ path to filesystem path
		fsPath := filepath.Join(outputDir, filepath.FromSlash(mpqPath))

		// Create directory
		if err := os.MkdirAll(filepath.Dir(fsPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", mpqPath, err)
		}

		// Extract file
		data, err := archive.ReadFile(mpqPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", mpqPath, err)
		}

		if err := os.WriteFile(fsPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fsPath, err)
		}
	}

	return nil
}

// ExtractDBCs extracts all DBC files from an archive to a directory
func ExtractDBCs(archivePath, outputDir string) error {
	archive, err := Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer archive.Close()

	dbcFiles, err := archive.GetDBCFiles()
	if err != nil {
		return fmt.Errorf("failed to get DBC files: %w", err)
	}

	for _, mpqPath := range dbcFiles {
		fsPath := filepath.Join(outputDir, filepath.Base(mpqPath))

		data, err := archive.ReadFile(mpqPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", mpqPath, err)
		}

		if err := os.WriteFile(fsPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fsPath, err)
		}
	}

	return nil
}

// bytesReadCloser wraps a byte slice as an io.ReadCloser
type bytesReadCloser struct {
	data []byte
	pos  int
}

func (b *bytesReadCloser) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bytesReadCloser) Close() error {
	return nil
}
