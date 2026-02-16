// Copyright (c) 2025 suprsokr
// SPDX-License-Identifier: MIT

// Package vanillampq provides a pure-Go MPQ V1 archive reader/writer
// for World of Warcraft Vanilla (1.0.0-1.12.1).
package vanillampq

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Archive represents a vanilla MPQ archive.
type Archive struct {
	file         *os.File
	path         string
	tempPath     string
	mode         string // "r" for read, "w" for write, "m" for modify
	header       *archiveHeader
	hashTable    []hashTableEntry
	blockTable   []blockTableEntry
	pendingFiles []pendingFile
	removedFiles map[string]bool
	sectorSize   uint32
}

type pendingFile struct {
	mpqPath        string
	data           []byte
	generateCRC    bool
	isPatchFile    bool
	isDeleteMarker bool
}

// Create creates a new vanilla-compatible MPQ V1 archive.
func Create(path string, maxFiles int) (*Archive, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, "mpq_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	hashTableSize := nextPowerOf2(uint32(float64(maxFiles) * 1.5))
	if hashTableSize < 16 {
		hashTableSize = 16
	}

	h := &archiveHeader{
		header: header{
			Magic:           mpqMagic,
			HeaderSize:      headerSizeV1,
			FormatVersion:   0, // V1
			SectorSizeShift: defaultSectorSizeShift,
			HashTableSize:   hashTableSize,
			BlockTableSize:  0,
		},
	}

	return &Archive{
		path:         path,
		tempPath:     tempPath,
		mode:         "w",
		header:       h,
		hashTable:    make([]hashTableEntry, hashTableSize),
		blockTable:   make([]blockTableEntry, 0, maxFiles),
		pendingFiles: make([]pendingFile, 0, maxFiles),
		removedFiles: make(map[string]bool),
		sectorSize:   defaultSectorSize,
	}, nil
}

// Open opens an existing MPQ V1 archive for reading.
func Open(path string) (*Archive, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	h, err := findArchiveHeader(file)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("read header: %w", err)
	}
	if h.Magic != mpqMagic {
		file.Close()
		return nil, fmt.Errorf("invalid MPQ magic: 0x%08X", h.Magic)
	}
	if h.FormatVersion != 0 {
		file.Close()
		return nil, fmt.Errorf("unsupported MPQ format version %d: vanillampq only supports V1", h.FormatVersion)
	}

	// Read hash table
	hashTableOffset := int64(h.HashTableOffset) + int64(h.ArchiveOffset)
	if _, err := file.Seek(hashTableOffset, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("seek to hash table: %w", err)
	}

	hashTableData := make([]uint32, h.HashTableSize*4)
	if err := binary.Read(file, binary.LittleEndian, hashTableData); err != nil {
		file.Close()
		return nil, fmt.Errorf("read hash table: %w", err)
	}
	decryptBlock(hashTableData, hashString("(hash table)", hashTypeFileKey))

	hashTable := make([]hashTableEntry, h.HashTableSize)
	for i := range hashTable {
		hashTable[i] = hashTableEntry{
			HashA:      hashTableData[i*4],
			HashB:      hashTableData[i*4+1],
			Locale:     uint16(hashTableData[i*4+2] & 0xFFFF),
			Platform:   uint16(hashTableData[i*4+2] >> 16),
			BlockIndex: hashTableData[i*4+3],
		}
	}

	// Read block table
	blockTableOffset := int64(h.BlockTableOffset) + int64(h.ArchiveOffset)
	if _, err := file.Seek(blockTableOffset, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("seek to block table: %w", err)
	}

	blockTableData := make([]uint32, h.BlockTableSize*4)
	if err := binary.Read(file, binary.LittleEndian, blockTableData); err != nil {
		file.Close()
		return nil, fmt.Errorf("read block table: %w", err)
	}
	decryptBlock(blockTableData, hashString("(block table)", hashTypeFileKey))

	blockTable := make([]blockTableEntry, h.BlockTableSize)
	for i := range blockTable {
		blockTable[i] = blockTableEntry{
			FilePos:        blockTableData[i*4],
			CompressedSize: blockTableData[i*4+1],
			FileSize:       blockTableData[i*4+2],
			Flags:          blockTableData[i*4+3],
		}
	}

	return &Archive{
		file:       file,
		path:       path,
		mode:       "r",
		header:     h,
		hashTable:  hashTable,
		blockTable: blockTable,
		sectorSize: 512 << h.SectorSizeShift,
	}, nil
}

// OpenForModify opens an existing archive for modification.
func OpenForModify(path string) (*Archive, error) {
	a, err := Open(path)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, "mpq_*.tmp")
	if err != nil {
		a.file.Close()
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	a.mode = "m"
	a.tempPath = tempPath
	a.pendingFiles = make([]pendingFile, 0)
	a.removedFiles = make(map[string]bool)
	return a, nil
}

// AddFile adds a file to the archive from a local path.
func (a *Archive) AddFile(srcPath, mpqPath string) error {
	return a.AddFileWithOptions(srcPath, mpqPath, false)
}

// AddFileWithCRC adds a file with sector CRC generation.
func (a *Archive) AddFileWithCRC(srcPath, mpqPath string) error {
	return a.AddFileWithOptions(srcPath, mpqPath, true)
}

// AddFileWithOptions adds a file with specified options.
func (a *Archive) AddFileWithOptions(srcPath, mpqPath string, generateCRC bool) error {
	if a.mode != "w" && a.mode != "m" {
		return fmt.Errorf("archive not opened for writing")
	}
	mpqPath = NormalizePath(mpqPath)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", srcPath, err)
	}
	a.pendingFiles = append(a.pendingFiles, pendingFile{
		mpqPath:     mpqPath,
		data:        data,
		generateCRC: generateCRC,
	})
	return nil
}

// AddFileFromBytes adds a file from a byte slice.
func (a *Archive) AddFileFromBytes(data []byte, mpqPath string) error {
	if a.mode != "w" && a.mode != "m" {
		return fmt.Errorf("archive not opened for writing")
	}
	mpqPath = NormalizePath(mpqPath)
	a.pendingFiles = append(a.pendingFiles, pendingFile{
		mpqPath: mpqPath,
		data:    data,
	})
	return nil
}

// AddPatchFile adds a file marked as a patch file.
func (a *Archive) AddPatchFile(srcPath, mpqPath string) error {
	if a.mode != "w" && a.mode != "m" {
		return fmt.Errorf("archive not opened for writing")
	}
	mpqPath = NormalizePath(mpqPath)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", srcPath, err)
	}
	a.pendingFiles = append(a.pendingFiles, pendingFile{
		mpqPath:     mpqPath,
		data:        data,
		isPatchFile: true,
	})
	return nil
}

// AddDeleteMarker adds a deletion marker for a file.
func (a *Archive) AddDeleteMarker(mpqPath string) error {
	if a.mode != "w" && a.mode != "m" {
		return fmt.Errorf("archive not opened for writing")
	}
	mpqPath = NormalizePath(mpqPath)
	a.pendingFiles = append(a.pendingFiles, pendingFile{
		mpqPath:        mpqPath,
		isDeleteMarker: true,
	})
	return nil
}

// RemoveFile marks a file for removal (modify mode only).
func (a *Archive) RemoveFile(mpqPath string) error {
	if a.mode != "m" {
		return fmt.Errorf("archive not opened for modification")
	}
	mpqPath = NormalizePath(mpqPath)
	if !a.HasFile(mpqPath) {
		return fmt.Errorf("file not found: %s", mpqPath)
	}
	a.removedFiles[mpqPath] = true
	return nil
}

// ExtractFile extracts a file from the archive to disk.
func (a *Archive) ExtractFile(mpqPath, destPath string) error {
	data, err := a.ReadFile(mpqPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return os.WriteFile(destPath, data, 0644)
}

// ReadFile reads a file from the archive into memory.
func (a *Archive) ReadFile(mpqPath string) ([]byte, error) {
	if a.mode != "r" && a.mode != "m" {
		return nil, fmt.Errorf("archive not opened for reading")
	}
	mpqPath = NormalizePath(mpqPath)

	block, err := a.findFile(mpqPath)
	if err != nil {
		return nil, err
	}

	filePos := int64(block.FilePos) + int64(a.header.ArchiveOffset)
	if _, err := a.file.Seek(filePos, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to file data: %w", err)
	}
	compressedData := make([]byte, block.CompressedSize)
	if _, err := io.ReadFull(a.file, compressedData); err != nil {
		return nil, fmt.Errorf("read file data: %w", err)
	}

	// Encrypted file
	if block.Flags&fileEncrypted != 0 {
		encryptionKey := getFileKey(mpqPath, block.FilePos, block.FileSize, block.Flags)
		if block.Flags&fileSingleUnit != 0 {
			return a.decryptAndDecompressSingleUnit(compressedData, block, encryptionKey)
		}
		return a.decryptAndDecompressSectors(compressedData, block, encryptionKey)
	}

	// Compressed file
	if block.Flags&fileCompress != 0 {
		if block.Flags&fileSingleUnit != 0 {
			dataToDecompress := compressedData
			if block.Flags&fileSectorCRC != 0 && len(compressedData) >= 4 {
				dataToDecompress = compressedData[:len(compressedData)-4]
			}
			if block.CompressedSize < block.FileSize {
				return decompressData(dataToDecompress, block.FileSize)
			}
			return dataToDecompress, nil
		}
		return a.decompressSectors(compressedData, block)
	}

	// Uncompressed
	if block.Flags&fileSingleUnit != 0 && block.Flags&fileSectorCRC != 0 && len(compressedData) >= 4 {
		return compressedData[:len(compressedData)-4], nil
	}
	return compressedData, nil
}

// ListFiles returns all files by reading the (listfile).
func (a *Archive) ListFiles() ([]string, error) {
	if a.mode != "r" && a.mode != "m" {
		return nil, fmt.Errorf("archive not opened for reading")
	}
	data, err := a.ReadFile("(listfile)")
	if err != nil {
		return nil, fmt.Errorf("read listfile: %w", err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")

	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "(listfile)" {
			files = append(files, line)
		}
	}
	return files, nil
}

// HasFile returns true if the archive contains the specified file.
func (a *Archive) HasFile(mpqPath string) bool {
	if a.mode == "w" {
		mpqPath = NormalizePath(mpqPath)
		for _, f := range a.pendingFiles {
			if strings.EqualFold(f.mpqPath, mpqPath) {
				return !f.isDeleteMarker
			}
		}
		return false
	}
	block, err := a.findFile(mpqPath)
	if err != nil {
		return false
	}
	return block.Flags&fileDeleteMarker == 0
}

// IsDeleteMarker returns true if the file is a deletion marker.
func (a *Archive) IsDeleteMarker(mpqPath string) bool {
	if a.mode != "r" {
		return false
	}
	block, err := a.findFile(mpqPath)
	if err != nil {
		return false
	}
	return block.Flags&fileDeleteMarker != 0
}

// IsPatchFile returns true if the file is marked as a patch file.
func (a *Archive) IsPatchFile(mpqPath string) bool {
	if a.mode != "r" {
		return false
	}
	block, err := a.findFile(mpqPath)
	if err != nil {
		return false
	}
	return block.Flags&filePatchFile != 0
}

// FileFilter is a function that filters files by name
type FileFilter func(path string) bool

// GetFilesByExtension returns all files with the given extension (e.g., ".dbc")
func (a *Archive) GetFilesByExtension(ext string) ([]string, error) {
	return a.GetFilesWithFilter(func(path string) bool {
		return strings.HasSuffix(strings.ToLower(path), strings.ToLower(ext))
	})
}

// GetFilesByPattern returns all files matching a pattern (case-insensitive)
func (a *Archive) GetFilesByPattern(pattern string) ([]string, error) {
	pattern = strings.ToLower(pattern)
	return a.GetFilesWithFilter(func(path string) bool {
		return strings.Contains(strings.ToLower(path), pattern)
	})
}

// GetFilesWithFilter returns all files matching the given filter function
func (a *Archive) GetFilesWithFilter(filter FileFilter) ([]string, error) {
	files, err := a.ListFiles()
	if err != nil {
		return nil, err
	}
	var filtered []string
	for _, f := range files {
		if filter(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

// GetDBCFiles returns all .dbc files in the archive.
// Deprecated: Use GetFilesByExtension(".dbc") instead.
func (a *Archive) GetDBCFiles() ([]string, error) {
	return a.GetFilesByExtension(".dbc")
}

// Close closes the archive. For write/modify mode, this writes the archive to disk.
func (a *Archive) Close() error {
	if a.mode == "r" {
		if a.file != nil {
			return a.file.Close()
		}
		return nil
	}

	if a.mode == "m" {
		if err := a.buildModifiedFileList(); err != nil {
			if a.file != nil {
				a.file.Close()
			}
			os.Remove(a.tempPath)
			return err
		}
		if a.file != nil {
			a.file.Close()
			a.file = nil
		}
	}

	if err := a.writeArchive(); err != nil {
		os.Remove(a.tempPath)
		return err
	}

	os.Remove(a.path)
	if err := os.Rename(a.tempPath, a.path); err != nil {
		if err2 := copyFile(a.tempPath, a.path); err2 != nil {
			os.Remove(a.tempPath)
			return fmt.Errorf("save archive: %w", err2)
		}
		os.Remove(a.tempPath)
	}
	return nil
}

// Path returns the archive file path.
func (a *Archive) Path() string {
	return a.path
}

// NormalizePath converts forward slashes to backslashes.
func NormalizePath(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}

// --- internal methods ---

func (a *Archive) findFile(mpqPath string) (*blockTableEntry, error) {
	mpqPath = NormalizePath(mpqPath)
	hashA := hashString(mpqPath, hashTypeNameA)
	hashB := hashString(mpqPath, hashTypeNameB)
	startIndex := hashString(mpqPath, hashTypeTableOffset) % a.header.HashTableSize

	for i := uint32(0); i < a.header.HashTableSize; i++ {
		idx := (startIndex + i) % a.header.HashTableSize
		entry := &a.hashTable[idx]
		if entry.BlockIndex == hashTableEmpty {
			break
		}
		if entry.BlockIndex == hashTableDeleted {
			continue
		}
		if entry.HashA == hashA && entry.HashB == hashB {
			if entry.BlockIndex < uint32(len(a.blockTable)) {
				block := &a.blockTable[entry.BlockIndex]
				if block.Flags&fileExists != 0 {
					return block, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("file not found: %s", mpqPath)
}

func (a *Archive) decryptAndDecompressSingleUnit(data []byte, block *blockTableEntry, key uint32) ([]byte, error) {
	decryptBytes(data, key)
	if block.Flags&fileCompress != 0 && block.CompressedSize < block.FileSize {
		return decompressData(data, block.FileSize)
	}
	if block.Flags&fileSectorCRC != 0 && len(data) >= 4 {
		return data[:len(data)-4], nil
	}
	return data, nil
}

func (a *Archive) decryptAndDecompressSectors(data []byte, block *blockTableEntry, key uint32) ([]byte, error) {
	numSectors := (block.FileSize + a.sectorSize - 1) / a.sectorSize
	offsetTableSize := (numSectors + 1) * 4
	if uint32(len(data)) < offsetTableSize {
		return nil, fmt.Errorf("data too small for sector offset table")
	}

	offsetTable := make([]uint32, numSectors+1)
	for i := range offsetTable {
		offsetTable[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	decryptBlock(offsetTable, key-1)

	result := make([]byte, 0, block.FileSize)
	for i := uint32(0); i < numSectors; i++ {
		sectorStart := offsetTable[i]
		sectorEnd := offsetTable[i+1]
		if sectorStart > uint32(len(data)) || sectorEnd > uint32(len(data)) || sectorEnd < sectorStart {
			return nil, fmt.Errorf("invalid sector offsets: %d-%d", sectorStart, sectorEnd)
		}

		sectorData := make([]byte, sectorEnd-sectorStart)
		copy(sectorData, data[sectorStart:sectorEnd])
		decryptBytes(sectorData, key+i)

		expectedSize := a.sectorSize
		if i == numSectors-1 {
			expectedSize = block.FileSize - (i * a.sectorSize)
		}

		if block.Flags&fileCompress != 0 && uint32(len(sectorData)) < expectedSize {
			decompressed, err := decompressData(sectorData, expectedSize)
			if err != nil {
				return nil, fmt.Errorf("decompress sector %d: %w", i, err)
			}
			result = append(result, decompressed...)
		} else {
			result = append(result, sectorData...)
		}
	}
	return result, nil
}

func (a *Archive) decompressSectors(data []byte, block *blockTableEntry) ([]byte, error) {
	numSectors := (block.FileSize + a.sectorSize - 1) / a.sectorSize
	offsetTableSize := (numSectors + 1) * 4
	if uint32(len(data)) < offsetTableSize {
		return nil, fmt.Errorf("data too small for sector offset table")
	}

	offsetTable := make([]uint32, numSectors+1)
	for i := range offsetTable {
		offsetTable[i] = binary.LittleEndian.Uint32(data[i*4:])
	}

	result := make([]byte, 0, block.FileSize)
	for i := uint32(0); i < numSectors; i++ {
		sectorStart := offsetTable[i]
		sectorEnd := offsetTable[i+1]
		if sectorStart > uint32(len(data)) || sectorEnd > uint32(len(data)) || sectorEnd < sectorStart {
			return nil, fmt.Errorf("invalid sector offsets: %d-%d (data len %d)", sectorStart, sectorEnd, len(data))
		}

		sectorData := data[sectorStart:sectorEnd]
		expectedSize := a.sectorSize
		if i == numSectors-1 {
			expectedSize = block.FileSize - (i * a.sectorSize)
		}

		if uint32(len(sectorData)) < expectedSize {
			decompressed, err := decompressData(sectorData, expectedSize)
			if err != nil {
				return nil, fmt.Errorf("decompress sector %d: %w", i, err)
			}
			result = append(result, decompressed...)
		} else {
			result = append(result, sectorData...)
		}
	}
	return result, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
