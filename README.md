# vanillampq

[![Go Reference](https://pkg.go.dev/badge/github.com/suprsokr/vanillampq.svg)](https://pkg.go.dev/github.com/suprsokr/vanillampq)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Pure-Go MPQ V1 archive library for World of Warcraft Vanilla (1.0.0-1.12.1).**

## Features

- **100% Pure Go** - Zero dependencies, stdlib only
- **Vanilla-Focused** - Specifically designed for vanilla WoW (1.0.0-1.12.1) v1 MPQ format
- **Streaming API** - Extract files without intermediate storage
- **Path Normalization** - Automatic backslash conversion (vanilla convention)
- **Full Read/Write Support** - Create, read, modify archives

## Installation

```bash
go get github.com/suprsokr/vanillampq
```

## Quick Start

### Opening and Listing Files

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/suprsokr/vanillampq"
)

func main() {
    // Open an archive
    archive, err := vanillampq.Open("dbc.MPQ")
    if err != nil {
        log.Fatal(err)
    }
    defer archive.Close()
    
    // List all files
    files, err := archive.ListFiles()
    if err != nil {
        log.Fatal(err)
    }
    
    for _, file := range files {
        fmt.Println(file)
    }
}
```

### Extracting Files

```go
// Extract all files to a directory
err := vanillampq.ExtractAll("dbc.MPQ", "./extracted")

// Extract only DBC files
err := vanillampq.ExtractDBCs("dbc.MPQ", "./dbcs")
```

### Streaming Files (No Intermediate Storage)

```go
entries, errors := vanillampq.StreamExtract("dbc.MPQ")

for entry := range entries {
    fmt.Printf("File: %s (%d bytes)\n", entry.Path, entry.Size)
    
    // Process the file reader directly
    data, _ := io.ReadAll(entry.Reader)
    entry.Reader.Close()
    
    // Process data...
}

if err := <-errors; err != nil {
    log.Fatal(err)
}
```

### Streaming Files by Extension

```go
// Stream only DBC files
entries, errors := vanillampq.StreamExtractByExtension("dbc.MPQ", ".dbc")

for entry := range entries {
    fmt.Printf("DBC: %s\n", entry.Path)
    // Process...
}

// Stream LUA files
luaEntries, luaErrors := vanillampq.StreamExtractByExtension("interface.MPQ", ".lua")

// Stream with custom filter
filter := func(path string) bool {
    return strings.HasPrefix(strings.ToLower(path), "dbfilesclient\\")
}
dbcEntries, dbcErrors := vanillampq.StreamExtractWithFilter("dbc.MPQ", filter)
```

### Creating Archives

```go
// Create a new archive
archive, err := vanillampq.Create("patch.MPQ", 100)
if err != nil {
    log.Fatal(err)
}
defer archive.Close()

// Add files (paths automatically normalized to backslashes)
err = archive.AddFile("local/spell.dbc", "DBFilesClient\\Spell.dbc")

// Add from bytes
data := []byte("...")
err = archive.AddFileFromBytes(data, "Interface\\FrameXML\\MyAddon.lua")
```

### Modifying Existing Archives

```go
archive, err := vanillampq.OpenForModify("patch.MPQ")
if err != nil {
    log.Fatal(err)
}
defer archive.Close()

// Add new file
archive.AddFile("local/newfile.dbc", "DBFilesClient\\NewFile.dbc")

// Remove file
archive.RemoveFile("DBFilesClient\\OldFile.dbc")

// Changes saved on Close()
```

## API Reference

### Core Functions

- `Open(path string) (*Archive, error)` - Open archive for reading
- `Create(path string, maxFiles int) (*Archive, error)` - Create new archive
- `OpenForModify(path string) (*Archive, error)` - Open for modification
- `ExtractAll(archivePath, outputDir string) error` - Extract all files
- `ExtractByExtension(archivePath, outputDir, ext string, preservePath bool) error` - Extract by extension
- `ExtractWithFilter(archivePath, outputDir string, filter FileFilter, preservePath bool) error` - Extract with custom filter

### Streaming Functions

- `StreamExtract(archivePath string) (<-chan FileEntry, <-chan error)` - Stream all files
- `StreamExtractByExtension(archivePath, ext string) (<-chan FileEntry, <-chan error)` - Stream files by extension
- `StreamExtractWithFilter(archivePath string, filter FileFilter) (<-chan FileEntry, <-chan error)` - Stream with custom filter

### Archive Methods

- `ListFiles() ([]string, error)` - List all file paths
- `GetFilesByExtension(ext string) ([]string, error)` - List files by extension (e.g., ".dbc", ".lua")
- `GetFilesByPattern(pattern string) ([]string, error)` - List files matching pattern
- `GetFilesWithFilter(filter FileFilter) ([]string, error)` - List files with custom filter
- `HasFile(path string) bool` - Check if file exists
- `ReadFile(path string) ([]byte, error)` - Read file into memory
- `ExtractFile(mpqPath, destPath string) error` - Extract single file
- `AddFile(localPath, mpqPath string) error` - Add file from disk
- `AddFileFromBytes(data []byte, mpqPath string) error` - Add file from bytes
- `RemoveFile(mpqPath string) error` - Remove file (modify mode only)
- `Close() error` - Close archive

### Utilities

- `NormalizePath(path string) string` - Convert slashes to backslashes
- `IsVanillaArchive(name string) bool` - Check if known vanilla archive
- `GetArchiveType(name string) VanillaArchiveType` - Get archive type

## Vanilla Archive Types

The library recognizes these standard vanilla MPQ archives:

**Base Archives:**
- `base.MPQ`, `dbc.MPQ`, `fonts.MPQ`, `interface.MPQ`
- `misc.MPQ`, `model.MPQ`, `sound.MPQ`, `speech.MPQ`
- `terrain.MPQ`, `texture.MPQ`, `wmo.MPQ`

**Patch Archives:**
- `patch.MPQ`, `patch-2.MPQ`, `patch-3.MPQ`

**Locale Archives:**
- `locale-enUS.MPQ`, `locale-deDE.MPQ`, `locale-frFR.MPQ`
- And other locale variants

## Pipeline Example

Combine with [vanilladbc](https://github.com/suprsokr/vanilladbc) for complete workflow:

```go
// Extract DBC → Convert → Edit → Repackage
entries, _ := vanillampq.StreamExtractDBCs("dbc.MPQ")

for entry := range entries {
    // Read DBC data
    dbcData, _ := io.ReadAll(entry.Reader)
    entry.Reader.Close()
    
    // Convert using vanilladbc
    // Edit data
    // Convert back
    // Add to new archive
}
```

## Command-Line Tool

For CLI usage, see [vanillampq-cli](https://github.com/suprsokr/vanillampq-cli).

## Implementation

This is a **native MPQ V1 implementation** written from scratch in pure Go with zero dependencies. The implementation was ported from:

- [go-mpq](https://github.com/JoshVarga/go-mpq) - Reference implementation
- [warcraft-rs](https://github.com/warlockbrawl/warcraft-rs) - Rust MPQ library

### What's Implemented

- MPQ V1 header parsing and writing
- Encrypted hash tables (StormLib-compatible hashing)
- Encrypted block tables
- Zlib compression/decompression
- PKWare DCL decompression
- Sector-based and single-unit file storage
- Sector CRC validation (Adler-32)
- File encryption/decryption
- Archive modification (add/remove/replace files)
- Deletion markers and patch file markers
- `(listfile)` and `(attributes)` support
- Streaming extraction API

### What's NOT Implemented

- MPQ V2, V3, V4 formats (TBC, WotLK, Cataclysm+)
- User data blocks
- Strong digital signatures
- Weak digital signatures
- BZip2 compression (not used in vanilla)
- Huffman encoding (not used in vanilla)
- Sparse/differential compression

## Testing

```bash
go test -v ./...
```

22 tests covering:
- Hash functions (StormLib compatibility)
- Encryption/decryption round-trips
- Create, read, modify archives
- Large file handling with sectors
- Path normalization
- CRC32 and Adler32 algorithms
- Test vectors from go-mpq and warcraft-rs

## Related Projects

- [vanillampq-cli](https://github.com/suprsokr/vanillampq-cli) - CLI tool
- [vanilladbc](https://github.com/suprsokr/vanilladbc) - DBC file library
- [WoWVanillaDBDefs](https://github.com/suprsokr/VanillaDBDefs) - Database definitions

## License

MIT License - see [LICENSE](LICENSE) for details.

## Credits

Implementation ported from:
- [go-mpq](https://github.com/JoshVarga/go-mpq) by Josh Varga
- [warcraft-rs](https://github.com/warlockbrawl/warcraft-rs) MPQ implementation

Inspired by the WoW modding community and StormLib.
