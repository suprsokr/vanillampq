// Copyright (c) 2025 suprsokr
// SPDX-License-Identifier: MIT

package vanillampq

// adler32 computes Adler-32 checksum (used for MPQ sector CRCs)
func adler32(data []byte) uint32 {
	const mod = 65521
	var a uint32 = 1
	var b uint32
	for _, v := range data {
		a = (a + uint32(v)) % mod
		b = (b + a) % mod
	}
	return (b << 16) | a
}

var crc32Table = func() [256]uint32 {
	var table [256]uint32
	const poly = 0xEDB88320
	for i := 0; i < 256; i++ {
		crc := uint32(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		table[i] = crc
	}
	return table
}()

func crc32Sum(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, v := range data {
		crc = crc32Table[(crc^uint32(v))&0xFF] ^ (crc >> 8)
	}
	return ^crc
}
