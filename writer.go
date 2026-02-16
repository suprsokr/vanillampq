// Copyright (c) 2025 suprsokr
// SPDX-License-Identifier: MIT

package vanillampq

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// writeArchive writes the complete MPQ V1 archive
func (a *Archive) writeArchive() error {
	file, err := os.Create(a.tempPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer file.Close()

	// Initialize hash table with empty entries
	for i := range a.hashTable {
		a.hashTable[i] = hashTableEntry{
			HashA:      0xFFFFFFFF,
			HashB:      0xFFFFFFFF,
			Locale:     0xFFFF,
			Platform:   0xFFFF,
			BlockIndex: hashTableEmpty,
		}
	}

	// Reserve space for header
	if _, err := file.Seek(int64(headerSizeV1), 0); err != nil {
		return fmt.Errorf("seek past header: %w", err)
	}

	actualFileCount := len(a.pendingFiles)
	totalBlockCount := actualFileCount
	if actualFileCount > 0 {
		totalBlockCount += 2 // (listfile) + (attributes)
	}

	a.blockTable = make([]blockTableEntry, 0, totalBlockCount)
	listFileContent := ""
	attributes := newAttributesWriter(totalBlockCount)

	for i, pf := range a.pendingFiles {
		filePos, _ := file.Seek(0, 1)

		// Deletion markers
		if pf.isDeleteMarker {
			a.blockTable = append(a.blockTable, blockTableEntry{
				FilePos: uint32(filePos), CompressedSize: 0, FileSize: 0,
				Flags: fileDeleteMarker | fileExists,
			})
			if err := a.addToHashTable(pf.mpqPath, uint32(len(a.blockTable)-1)); err != nil {
				return err
			}
			listFileContent += pf.mpqPath + "\r\n"
			continue
		}

		var dataToWrite []byte
		var flags uint32 = fileExists
		var compressedSize uint32
		useSectors := len(pf.data) > int(a.sectorSize)*2

		if useSectors {
			dataToWrite, compressedSize, err = a.writeSectoredFile(pf.data, pf.generateCRC)
			if err != nil {
				return fmt.Errorf("write sectored file %s: %w", pf.mpqPath, err)
			}
			flags |= fileCompress
			if pf.generateCRC {
				flags |= fileSectorCRC
			}
		} else {
			compressedData, err := compressData(pf.data)
			if err != nil {
				return fmt.Errorf("compress file %s: %w", pf.mpqPath, err)
			}
			flags |= fileSingleUnit
			if len(compressedData) < len(pf.data) {
				dataToWrite = compressedData
				flags |= fileCompress
			} else {
				dataToWrite = pf.data
			}
			if pf.generateCRC {
				crc := adler32(dataToWrite)
				crcBytes := make([]byte, 4)
				binary.LittleEndian.PutUint32(crcBytes, crc)
				dataToWrite = append(dataToWrite, crcBytes...)
				flags |= fileSectorCRC
			}
			compressedSize = uint32(len(dataToWrite))
		}

		if pf.isPatchFile {
			flags |= filePatchFile
		}

		if _, err := file.Write(dataToWrite); err != nil {
			return fmt.Errorf("write file data: %w", err)
		}

		a.blockTable = append(a.blockTable, blockTableEntry{
			FilePos: uint32(filePos), CompressedSize: compressedSize,
			FileSize: uint32(len(pf.data)), Flags: flags,
		})
		attributes.setEntry(i, pf.data)

		if err := a.addToHashTable(pf.mpqPath, uint32(len(a.blockTable)-1)); err != nil {
			return err
		}
		listFileContent += pf.mpqPath + "\r\n"
	}

	// Add (listfile)
	if listFileContent != "" {
		listFileData := []byte(listFileContent)
		listFilePos, _ := file.Seek(0, 1)

		compressedLF, _ := compressData(listFileData)
		var lfData []byte
		var lfFlags uint32 = fileExists | fileSingleUnit
		if len(compressedLF) < len(listFileData) {
			lfData = compressedLF
			lfFlags |= fileCompress
		} else {
			lfData = listFileData
		}

		file.Write(lfData)
		a.blockTable = append(a.blockTable, blockTableEntry{
			FilePos: uint32(listFilePos), CompressedSize: uint32(len(lfData)),
			FileSize: uint32(len(listFileData)), Flags: lfFlags,
		})
		attributes.setEntry(len(a.pendingFiles), listFileData)
		a.addToHashTable("(listfile)", uint32(len(a.blockTable)-1))
	}

	// Add (attributes)
	attrIdx := len(a.pendingFiles)
	if listFileContent != "" {
		attrIdx++
	}
	attributes.setEntry(attrIdx, nil)

	attrData, _ := attributes.build()
	if len(attrData) > 0 {
		attrPos, _ := file.Seek(0, 1)
		compressedAttr, _ := compressData(attrData)
		var aData []byte
		var aFlags uint32 = fileExists | fileSingleUnit
		if len(compressedAttr) < len(attrData) {
			aData = compressedAttr
			aFlags |= fileCompress
		} else {
			aData = attrData
		}

		file.Write(aData)
		a.blockTable = append(a.blockTable, blockTableEntry{
			FilePos: uint32(attrPos), CompressedSize: uint32(len(aData)),
			FileSize: uint32(len(attrData)), Flags: aFlags,
		})
		a.addToHashTable("(attributes)", uint32(len(a.blockTable)-1))
	}

	// Write hash table
	hashTableOffset, _ := file.Seek(0, 1)
	hashTableData := make([]uint32, len(a.hashTable)*4)
	for i, entry := range a.hashTable {
		hashTableData[i*4] = entry.HashA
		hashTableData[i*4+1] = entry.HashB
		hashTableData[i*4+2] = uint32(entry.Locale) | (uint32(entry.Platform) << 16)
		hashTableData[i*4+3] = entry.BlockIndex
	}
	encryptBlock(hashTableData, hashString("(hash table)", hashTypeFileKey))
	binary.Write(file, binary.LittleEndian, hashTableData)

	// Write block table
	blockTableOffset, _ := file.Seek(0, 1)
	blockTableData := make([]uint32, len(a.blockTable)*4)
	for i, entry := range a.blockTable {
		blockTableData[i*4] = entry.FilePos
		blockTableData[i*4+1] = entry.CompressedSize
		blockTableData[i*4+2] = entry.FileSize
		blockTableData[i*4+3] = entry.Flags
	}
	encryptBlock(blockTableData, hashString("(block table)", hashTypeFileKey))
	binary.Write(file, binary.LittleEndian, blockTableData)

	totalFileSize, _ := file.Seek(0, 1)
	archiveSize := uint32(totalFileSize) - headerSizeV1

	// Write header
	a.header.HashTableOffset = uint32(hashTableOffset)
	a.header.BlockTableOffset = uint32(blockTableOffset)
	a.header.BlockTableSize = uint32(len(a.blockTable))
	a.header.ArchiveSize = archiveSize

	file.Seek(0, 0)
	writeHeader(file, &a.header.header)

	return nil
}

func (a *Archive) writeSectoredFile(data []byte, useCRC bool) ([]byte, uint32, error) {
	numSectors := (uint32(len(data)) + a.sectorSize - 1) / a.sectorSize
	offsetTable := make([]uint32, numSectors+1)
	sectorCRCs := make([]uint32, 0, numSectors)
	sectors := make([][]byte, numSectors)

	offsetTableSize := (numSectors + 1) * 4
	var crcTableSize uint32
	if useCRC {
		crcTableSize = numSectors * 4
	}
	currentOffset := offsetTableSize + crcTableSize

	for i := uint32(0); i < numSectors; i++ {
		start := i * a.sectorSize
		end := start + a.sectorSize
		if end > uint32(len(data)) {
			end = uint32(len(data))
		}
		sectorData := data[start:end]
		compressed, err := compressData(sectorData)
		if err != nil {
			return nil, 0, fmt.Errorf("compress sector %d: %w", i, err)
		}
		if len(compressed) < len(sectorData) {
			sectors[i] = compressed
		} else {
			sectors[i] = sectorData
		}
		offsetTable[i] = currentOffset
		currentOffset += uint32(len(sectors[i]))
		if useCRC {
			sectorCRCs = append(sectorCRCs, adler32(sectorData))
		}
	}
	offsetTable[numSectors] = currentOffset

	result := make([]byte, currentOffset)
	offset := uint32(0)
	for _, off := range offsetTable {
		binary.LittleEndian.PutUint32(result[offset:], off)
		offset += 4
	}
	if useCRC {
		for _, crc := range sectorCRCs {
			binary.LittleEndian.PutUint32(result[offset:], crc)
			offset += 4
		}
	}
	for _, sector := range sectors {
		copy(result[offset:], sector)
		offset += uint32(len(sector))
	}
	return result, currentOffset, nil
}

func (a *Archive) addToHashTable(mpqPath string, blockIndex uint32) error {
	hashA := hashString(mpqPath, hashTypeNameA)
	hashB := hashString(mpqPath, hashTypeNameB)
	startIndex := hashString(mpqPath, hashTypeTableOffset) % a.header.HashTableSize

	for i := uint32(0); i < a.header.HashTableSize; i++ {
		idx := (startIndex + i) % a.header.HashTableSize
		entry := &a.hashTable[idx]
		if entry.BlockIndex == hashTableEmpty || entry.BlockIndex == hashTableDeleted {
			entry.HashA = hashA
			entry.HashB = hashB
			entry.Locale = localeNeutral
			entry.Platform = 0
			entry.BlockIndex = blockIndex
			return nil
		}
	}
	return fmt.Errorf("hash table full")
}

// buildModifiedFileList constructs the pending file list for modify mode
func (a *Archive) buildModifiedFileList() error {
	fileList, err := a.ListFiles()
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}

	pendingMap := make(map[string]pendingFile)
	for _, pf := range a.pendingFiles {
		pendingMap[NormalizePath(pf.mpqPath)] = pf
	}

	newPendingFiles := make([]pendingFile, 0)

	for _, mpqPath := range fileList {
		normalizedPath := NormalizePath(mpqPath)

		if a.removedFiles[normalizedPath] {
			continue
		}
		if normalizedPath == "(listfile)" || normalizedPath == "(attributes)" {
			continue
		}

		if pending, exists := pendingMap[normalizedPath]; exists {
			newPendingFiles = append(newPendingFiles, pending)
			delete(pendingMap, normalizedPath)
		} else {
			block, err := a.findFile(normalizedPath)
			if err != nil {
				continue
			}

			if block.Flags&fileDeleteMarker != 0 {
				newPendingFiles = append(newPendingFiles, pendingFile{
					mpqPath: normalizedPath, isDeleteMarker: true,
				})
				continue
			}

			// Extract existing file data
			extractedData, err := a.readFileFromBlock(normalizedPath, block)
			if err != nil {
				return fmt.Errorf("extract file %s: %w", normalizedPath, err)
			}

			newPendingFiles = append(newPendingFiles, pendingFile{
				mpqPath:     normalizedPath,
				data:        extractedData,
				generateCRC: block.Flags&fileSectorCRC != 0,
				isPatchFile: block.Flags&filePatchFile != 0,
			})
		}
	}

	for _, pending := range pendingMap {
		newPendingFiles = append(newPendingFiles, pending)
	}
	a.pendingFiles = newPendingFiles
	return nil
}

// readFileFromBlock reads raw file data for modification
func (a *Archive) readFileFromBlock(mpqPath string, block *blockTableEntry) ([]byte, error) {
	filePos := int64(block.FilePos) + int64(a.header.ArchiveOffset)
	if _, err := a.file.Seek(filePos, 0); err != nil {
		return nil, err
	}
	compressedData := make([]byte, block.CompressedSize)
	if _, err := a.file.Read(compressedData); err != nil {
		return nil, err
	}

	if block.Flags&fileEncrypted != 0 {
		key := hashString(filepath.Base(mpqPath), hashTypeFileKey)
		if block.Flags&fileFixKey != 0 {
			key = (key + block.FilePos) ^ block.FileSize
		}
		if block.Flags&fileSingleUnit != 0 {
			return a.decryptAndDecompressSingleUnit(compressedData, block, key)
		}
		return a.decryptAndDecompressSectors(compressedData, block, key)
	}

	if block.Flags&fileCompress != 0 {
		if block.Flags&fileSingleUnit != 0 {
			d := compressedData
			if block.Flags&fileSectorCRC != 0 && len(d) >= 4 {
				d = d[:len(d)-4]
			}
			if block.CompressedSize < block.FileSize {
				return decompressData(d, block.FileSize)
			}
			return d, nil
		}
		return a.decompressSectors(compressedData, block)
	}

	if block.Flags&fileSingleUnit != 0 && block.Flags&fileSectorCRC != 0 && len(compressedData) >= 4 {
		return compressedData[:len(compressedData)-4], nil
	}
	return compressedData, nil
}
