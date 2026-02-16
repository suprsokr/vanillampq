// Copyright (c) 2025 suprsokr
// SPDX-License-Identifier: MIT

package vanillampq

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// Compression type constants
const (
	compressionZlib   = 0x02
	compressionPKWare = 0x08
)

// compressData compresses data using zlib
func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(compressionZlib)

	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create zlib writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("zlib write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("zlib close: %w", err)
	}
	return buf.Bytes(), nil
}

// decompressData decompresses MPQ-compressed data
func decompressData(data []byte, uncompressedSize uint32) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty compressed data")
	}

	compressionType := data[0]
	data = data[1:]

	switch compressionType {
	case compressionZlib:
		return decompressZlib(data, uncompressedSize)
	case compressionPKWare:
		return decompressPKWare(data, uncompressedSize)
	default:
		// Multi-compression fallback
		if compressionType&compressionZlib != 0 {
			return decompressZlib(data, uncompressedSize)
		}
		if compressionType&compressionPKWare != 0 {
			return decompressPKWare(data, uncompressedSize)
		}
		return nil, fmt.Errorf("unsupported compression type: 0x%02X", compressionType)
	}
}

func decompressZlib(data []byte, uncompressedSize uint32) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create zlib reader: %w", err)
	}
	defer r.Close()

	result := make([]byte, uncompressedSize)
	n, err := io.ReadFull(r, result)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("zlib decompress: %w", err)
	}
	return result[:n], nil
}
