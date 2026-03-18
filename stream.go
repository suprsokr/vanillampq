// Copyright (c) 2025 suprsokr
// SPDX-License-Identifier: MIT

package vanillampq

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileEntry represents a file being streamed from an archive
type FileEntry struct {
	Path   string
	Size   int64
	Reader io.ReadCloser
}

// StreamExtract streams all files from an archive
func StreamExtract(archivePath string) (<-chan FileEntry, <-chan error) {
	entries := make(chan FileEntry)
	errors := make(chan error, 1)
	go func() {
		defer close(entries)
		defer close(errors)

		archive, err := Open(archivePath)
		if err != nil {
			errors <- fmt.Errorf("open archive: %w", err)
			return
		}
		defer archive.Close()

		files, err := archive.ListFiles()
		if err != nil {
			errors <- fmt.Errorf("list files: %w", err)
			return
		}

		for _, path := range files {
			data, err := archive.ReadFile(path)
			if err != nil {
				errors <- fmt.Errorf("read %s: %w", path, err)
				return
			}
			entries <- FileEntry{
				Path:   path,
				Size:   int64(len(data)),
				Reader: &bytesReadCloser{data: data},
			}
		}
	}()
	return entries, errors
}

// StreamExtractWithFilter streams files matching a filter function from an archive
func StreamExtractWithFilter(archivePath string, filter FileFilter) (<-chan FileEntry, <-chan error) {
	entries := make(chan FileEntry)
	errors := make(chan error, 1)
	go func() {
		defer close(entries)
		defer close(errors)

		archive, err := Open(archivePath)
		if err != nil {
			errors <- fmt.Errorf("open archive: %w", err)
			return
		}
		defer archive.Close()

		files, err := archive.ListFiles()
		if err != nil {
			errors <- fmt.Errorf("list files: %w", err)
			return
		}

		for _, path := range files {
			if filter != nil && !filter(path) {
				continue
			}
			data, err := archive.ReadFile(path)
			if err != nil {
				errors <- fmt.Errorf("read %s: %w", path, err)
				return
			}
			entries <- FileEntry{
				Path:   path,
				Size:   int64(len(data)),
				Reader: &bytesReadCloser{data: data},
			}
		}
	}()
	return entries, errors
}

// StreamExtractByExtension streams files with a specific extension (e.g., ".dbc", ".lua")
func StreamExtractByExtension(archivePath, ext string) (<-chan FileEntry, <-chan error) {
	ext = strings.ToLower(ext)
	return StreamExtractWithFilter(archivePath, func(path string) bool {
		return strings.HasSuffix(strings.ToLower(path), ext)
	})
}

// ExtractAll extracts all files from an archive to a directory
func ExtractAll(archivePath, outputDir string) error {
	archive, err := Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	files, err := archive.ListFiles()
	if err != nil {
		return err
	}

	for _, mpqPath := range files {
		fsPath := filepath.Join(outputDir, filepath.FromSlash(strings.ReplaceAll(mpqPath, "\\", "/")))
		if err := os.MkdirAll(filepath.Dir(fsPath), 0755); err != nil {
			return fmt.Errorf("create dir for %s: %w", mpqPath, err)
		}
		data, err := archive.ReadFile(mpqPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", mpqPath, err)
		}
		if err := os.WriteFile(fsPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", fsPath, err)
		}
	}
	return nil
}

// ExtractWithFilter extracts files matching a filter to a directory
func ExtractWithFilter(archivePath, outputDir string, filter FileFilter, preservePath bool) error {
	archive, err := Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	files, err := archive.ListFiles()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	var firstErr error
	for _, mpqPath := range files {
		if filter != nil && !filter(mpqPath) {
			continue
		}

		normalized := strings.ReplaceAll(mpqPath, "\\", "/")
		var fsPath string
		if preservePath {
			fsPath = filepath.Join(outputDir, filepath.FromSlash(normalized))
			if err := os.MkdirAll(filepath.Dir(fsPath), 0755); err != nil {
				return fmt.Errorf("create dir for %s: %w", mpqPath, err)
			}
		} else {
			fsPath = filepath.Join(outputDir, filepath.Base(normalized))
		}

		data, err := archive.ReadFile(mpqPath)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("read %s: %w", mpqPath, err)
			}
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", mpqPath, err)
			continue
		}
		if err := os.WriteFile(fsPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", fsPath, err)
		}
	}
	return nil
}

// ExtractByExtension extracts all files with a specific extension (e.g., ".dbc", ".lua")
func ExtractByExtension(archivePath, outputDir, ext string, preservePath bool) error {
	ext = strings.ToLower(ext)
	return ExtractWithFilter(archivePath, outputDir, func(path string) bool {
		return strings.HasSuffix(strings.ToLower(path), ext)
	}, preservePath)
}

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

func (b *bytesReadCloser) Close() error { return nil }
