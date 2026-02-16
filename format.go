// Copyright (c) 2025 suprsokr
// SPDX-License-Identifier: MIT

package vanillampq

import (
	"encoding/binary"
	"io"
)

// MPQ V1 format constants (vanilla WoW only)
const (
	mpqMagic         = 0x1A51504D // "MPQ\x1A"
	mpqUserDataMagic = 0x1B51504D // "MPQ\x1B"
	headerAlignment  = 0x200
	headerSizeV1     = 0x20 // 32 bytes

	// Block table entry flags
	fileImplode      = 0x00000100
	fileCompress     = 0x00000200
	fileEncrypted    = 0x00010000
	fileFixKey       = 0x00020000
	filePatchFile    = 0x00100000
	fileSingleUnit   = 0x01000000
	fileDeleteMarker = 0x02000000
	fileSectorCRC    = 0x04000000
	fileExists       = 0x80000000

	// Hash table entry constants
	hashTableEmpty   = 0xFFFFFFFF
	hashTableDeleted = 0xFFFFFFFE

	localeNeutral = 0x00000000

	defaultSectorSizeShift = 3
	defaultSectorSize      = 512 << defaultSectorSizeShift // 4096 bytes
)

// userDataHeader is the optional MPQ user data header (MPQ\x1B)
type userDataHeader struct {
	Magic              uint32
	UserDataSize       uint32
	HeaderOffset       uint32
	UserDataHeaderSize uint32
}

// header is the MPQ V1 archive header (32 bytes)
type header struct {
	Magic            uint32
	HeaderSize       uint32
	ArchiveSize      uint32
	FormatVersion    uint16
	SectorSizeShift  uint16
	HashTableOffset  uint32
	BlockTableOffset uint32
	HashTableSize    uint32
	BlockTableSize   uint32
}

// archiveHeader wraps the header with additional metadata
type archiveHeader struct {
	header
	ArchiveOffset uint64
	UserData      *userDataHeader
}

// hashTableEntry represents an entry in the hash table
type hashTableEntry struct {
	HashA      uint32
	HashB      uint32
	Locale     uint16
	Platform   uint16
	BlockIndex uint32
}

// blockTableEntry represents an entry in the block table
type blockTableEntry struct {
	FilePos        uint32
	CompressedSize uint32
	FileSize       uint32
	Flags          uint32
}

// findArchiveHeader scans for the MPQ header, handling user data headers
func findArchiveHeader(r io.ReadSeeker) (*archiveHeader, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var offset uint64
	buf := make([]byte, 4)
	for {
		if _, err := r.Seek(int64(offset), io.SeekStart); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		sig := binary.LittleEndian.Uint32(buf)
		switch sig {
		case mpqMagic:
			if _, err := r.Seek(int64(offset), io.SeekStart); err != nil {
				return nil, err
			}
			var h header
			if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
				return nil, err
			}
			return &archiveHeader{header: h, ArchiveOffset: offset}, nil
		case mpqUserDataMagic:
			var ud userDataHeader
			if err := binary.Read(r, binary.LittleEndian, &ud); err != nil {
				return nil, err
			}
			if ud.HeaderOffset == 0 {
				return nil, io.ErrUnexpectedEOF
			}
			archiveOffset := offset + uint64(ud.HeaderOffset)
			if _, err := r.Seek(int64(archiveOffset), io.SeekStart); err != nil {
				return nil, err
			}
			var h header
			if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
				return nil, err
			}
			if h.Magic != mpqMagic {
				return nil, io.ErrUnexpectedEOF
			}
			return &archiveHeader{header: h, ArchiveOffset: archiveOffset, UserData: &ud}, nil
		}
		offset += headerAlignment
	}
}

// writeHeader writes the V1 header to a writer
func writeHeader(w io.Writer, h *header) error {
	return binary.Write(w, binary.LittleEndian, h)
}

func nextPowerOf2(n uint32) uint32 {
	if n == 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}
