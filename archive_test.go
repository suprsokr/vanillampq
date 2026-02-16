// Copyright (c) 2025 suprsokr
// SPDX-License-Identifier: MIT

package vanillampq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndRead(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vanillampq_test_")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile1 := filepath.Join(tmpDir, "test1.txt")
	testFile2 := filepath.Join(tmpDir, "test2.txt")
	testContent1 := []byte("Hello, World! This is test file 1 with some content.")
	testContent2 := []byte("Test file 2 contains different data for the archive.")

	os.WriteFile(testFile1, testContent1, 0644)
	os.WriteFile(testFile2, testContent2, 0644)

	mpqPath := filepath.Join(tmpDir, "test.mpq")
	archive, err := Create(mpqPath, 10)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	if err := archive.AddFile(testFile1, "Data\\Test1.txt"); err != nil {
		t.Fatalf("add file 1: %v", err)
	}
	if err := archive.AddFile(testFile2, "Data\\SubDir\\Test2.txt"); err != nil {
		t.Fatalf("add file 2: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	if _, err := os.Stat(mpqPath); os.IsNotExist(err) {
		t.Fatalf("MPQ file not created")
	}

	readArchive, err := Open(mpqPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer readArchive.Close()

	if !readArchive.HasFile("Data\\Test1.txt") {
		t.Errorf("file 1 not found")
	}
	if !readArchive.HasFile("Data\\SubDir\\Test2.txt") {
		t.Errorf("file 2 not found")
	}
	if readArchive.HasFile("NonExistent.txt") {
		t.Errorf("non-existent file found")
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	extract1 := filepath.Join(extractDir, "test1.txt")
	extract2 := filepath.Join(extractDir, "test2.txt")

	if err := readArchive.ExtractFile("Data\\Test1.txt", extract1); err != nil {
		t.Fatalf("extract file 1: %v", err)
	}
	if err := readArchive.ExtractFile("Data\\SubDir\\Test2.txt", extract2); err != nil {
		t.Fatalf("extract file 2: %v", err)
	}

	extracted1, _ := os.ReadFile(extract1)
	if string(extracted1) != string(testContent1) {
		t.Errorf("file 1 mismatch: got %q, want %q", extracted1, testContent1)
	}
	extracted2, _ := os.ReadFile(extract2)
	if string(extracted2) != string(testContent2) {
		t.Errorf("file 2 mismatch: got %q, want %q", extracted2, testContent2)
	}
}

func TestHashString(t *testing.T) {
	tests := []struct {
		input    string
		hashType uint32
		expected uint32
	}{
		{"(hash table)", hashTypeFileKey, 0xC3AF3770},
		{"(block table)", hashTypeFileKey, 0xEC83B3A3},
	}
	for _, test := range tests {
		got := hashString(test.input, test.hashType)
		if got != test.expected {
			t.Errorf("hashString(%q, %d) = 0x%08X, want 0x%08X",
				test.input, test.hashType, got, test.expected)
		}
	}
}

func TestHashStringFromStormLib(t *testing.T) {
	tests := []struct {
		name  string
		input string
		hashA uint32
		hashB uint32
	}{
		{
			name: "StormLib test file path",
			input: "ReplaceableTextures\\CommandButtons\\BTNHaboss79.blp",
			hashA: 0x8bd6929a, hashB: 0xfd55129b,
		},
		{
			name: "Forward slashes normalized",
			input: "ReplaceableTextures/CommandButtons/BTNHaboss79.blp",
			hashA: 0x8bd6929a, hashB: 0xfd55129b,
		},
		{
			name: "Case insensitive",
			input: "replaceabletextures\\commandbuttons\\btnhaboss79.blp",
			hashA: 0x8bd6929a, hashB: 0xfd55129b,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotA := hashString(test.input, hashTypeNameA)
			gotB := hashString(test.input, hashTypeNameB)
			if gotA != test.hashA {
				t.Errorf("hashA = 0x%08X, want 0x%08X", gotA, test.hashA)
			}
			if gotB != test.hashB {
				t.Errorf("hashB = 0x%08X, want 0x%08X", gotB, test.hashB)
			}
		})
	}
}

func TestHashStringPathNormalization(t *testing.T) {
	// Verify forward and backslashes produce the same hash
	hash1 := hashString("path/to/file.txt", hashTypeTableOffset)
	hash2 := hashString("path\\to\\file.txt", hashTypeTableOffset)
	if hash1 != hash2 {
		t.Errorf("path normalization failed: 0x%08X != 0x%08X", hash1, hash2)
	}

	// Test vector from warcraft-rs (path\to\file without extension)
	hash3 := hashString("path\\to\\file", hashTypeTableOffset)
	hash4 := hashString("path/to/file", hashTypeTableOffset)
	if hash3 != hash4 {
		t.Errorf("path normalization failed: 0x%08X != 0x%08X", hash3, hash4)
	}
	if hash3 != 0x534CC8EE {
		t.Errorf("expected 0x534CC8EE, got 0x%08X", hash3)
	}
}

func TestHashStringCaseInsensitivity(t *testing.T) {
	hash1 := hashString("file.txt", hashTypeTableOffset)
	hash2 := hashString("FILE.TXT", hashTypeTableOffset)
	if hash1 != hash2 {
		t.Errorf("case insensitivity failed: 0x%08X != 0x%08X", hash1, hash2)
	}
	if hash1 != 0x3EA98D7A {
		t.Errorf("expected 0x3EA98D7A, got 0x%08X", hash1)
	}
}

func TestCryptTableInitialization(t *testing.T) {
	if len(cryptTable) != 0x500 {
		t.Errorf("cryptTable length = %d, want %d", len(cryptTable), 0x500)
	}
	seed := uint32(0x00100001)
	for index1 := 0; index1 < 0x100; index1++ {
		index2 := index1
		for i := 0; i < 5; i++ {
			seed = (seed*125 + 3) % 0x2AAAAB
			temp1 := (seed & 0xFFFF) << 0x10
			seed = (seed*125 + 3) % 0x2AAAAB
			temp2 := seed & 0xFFFF
			expected := temp1 | temp2
			if cryptTable[index2] != expected {
				t.Errorf("cryptTable[0x%03X] = 0x%08X, want 0x%08X", index2, cryptTable[index2], expected)
			}
			index2 += 0x100
		}
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	testCases := []struct {
		name string
		data []uint32
		key  string
	}{
		{"hash table key", []uint32{0x12345678, 0xDEADBEEF, 0xCAFEBABE, 0xF00DF00D}, "(hash table)"},
		{"block table key", []uint32{0x11111111, 0x22222222, 0x33333333, 0x44444444}, "(block table)"},
		{"single value", []uint32{0xABCDEF01}, "(hash table)"},
		{"zeros", []uint32{0, 0, 0, 0}, "(hash table)"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := make([]uint32, len(tc.data))
			copy(original, tc.data)
			data := make([]uint32, len(tc.data))
			copy(data, tc.data)
			key := hashString(tc.key, hashTypeFileKey)

			encryptBlock(data, key)
			decryptBlock(data, key)

			for i := range original {
				if data[i] != original[i] {
					t.Errorf("round-trip mismatch at %d: got 0x%08X, want 0x%08X", i, data[i], original[i])
				}
			}
		})
	}
}

func TestPathNormalization(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_path_test_")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	mpqPath := filepath.Join(tmpDir, "test.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFile(testFile, "Interface/AddOns/Test.lua")
	archive.Close()

	readArchive, _ := Open(mpqPath)
	defer readArchive.Close()

	if !readArchive.HasFile("Interface\\AddOns\\Test.lua") {
		t.Errorf("file not found with backslashes")
	}
	if !readArchive.HasFile("Interface/AddOns/Test.lua") {
		t.Errorf("file not found with forward slashes")
	}
}

func TestEmptyArchive(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_empty_test_")
	defer os.RemoveAll(tmpDir)

	mpqPath := filepath.Join(tmpDir, "empty.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.Close()

	if _, err := os.Stat(mpqPath); os.IsNotExist(err) {
		t.Fatalf("MPQ file not created")
	}
	readArchive, err := Open(mpqPath)
	if err != nil {
		t.Fatalf("open empty archive: %v", err)
	}
	defer readArchive.Close()

	if readArchive.HasFile("anything.txt") {
		t.Errorf("found file in empty archive")
	}
}

func TestLargeFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_large_test_")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "large.bin")
	largeData := make([]byte, 100*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	os.WriteFile(testFile, largeData, 0644)

	mpqPath := filepath.Join(tmpDir, "large.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFile(testFile, "Data\\Large.bin")
	archive.Close()

	readArchive, _ := Open(mpqPath)
	defer readArchive.Close()

	extractPath := filepath.Join(tmpDir, "extracted.bin")
	readArchive.ExtractFile("Data\\Large.bin", extractPath)

	extracted, _ := os.ReadFile(extractPath)
	if len(extracted) != len(largeData) {
		t.Fatalf("size mismatch: got %d, want %d", len(extracted), len(largeData))
	}
	for i := range largeData {
		if extracted[i] != largeData[i] {
			t.Fatalf("data mismatch at byte %d", i)
		}
	}
}

func TestV1HeaderSize(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_header_test_")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	mpqPath := filepath.Join(tmpDir, "v1.mpq")
	v1, _ := Create(mpqPath, 10)
	v1.AddFile(testFile, "test.txt")
	v1.Close()

	f, _ := os.Open(mpqPath)
	defer f.Close()
	hdr := make([]byte, 8)
	f.Read(hdr)
	hdrSize := uint32(hdr[4]) | uint32(hdr[5])<<8 | uint32(hdr[6])<<16 | uint32(hdr[7])<<24
	if hdrSize != 0x20 {
		t.Errorf("V1 header size: got 0x%X, want 0x20", hdrSize)
	}
}

func TestDeletionMarker(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_delete_test_")
	defer os.RemoveAll(tmpDir)

	mpqPath := filepath.Join(tmpDir, "delete.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddDeleteMarker("Data\\Deleted.txt")
	archive.Close()

	readArchive, _ := Open(mpqPath)
	defer readArchive.Close()

	if !readArchive.IsDeleteMarker("Data\\Deleted.txt") {
		t.Errorf("file not marked for deletion")
	}
	if readArchive.HasFile("Data\\Deleted.txt") {
		t.Errorf("HasFile returned true for deletion marker")
	}
}

func TestPatchFileMarker(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_patch_test_")
	defer os.RemoveAll(tmpDir)

	patchFile := filepath.Join(tmpDir, "patch.dat")
	os.WriteFile(patchFile, []byte("Patch data"), 0644)

	mpqPath := filepath.Join(tmpDir, "patch.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddPatchFile(patchFile, "Data\\Patch.dat")
	archive.Close()

	readArchive, _ := Open(mpqPath)
	defer readArchive.Close()

	if !readArchive.IsPatchFile("Data\\Patch.dat") {
		t.Errorf("file not marked as patch file")
	}
}

func TestModifyArchive(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_modify_test_")
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, []byte("Original content 1"), 0644)
	os.WriteFile(file2, []byte("Original content 2"), 0644)

	mpqPath := filepath.Join(tmpDir, "test.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFile(file1, "Data\\File1.txt")
	archive.AddFile(file2, "Data\\File2.txt")
	archive.Close()

	archive, err := OpenForModify(mpqPath)
	if err != nil {
		t.Fatalf("open for modify: %v", err)
	}

	file3 := filepath.Join(tmpDir, "file3.txt")
	os.WriteFile(file3, []byte("New file content"), 0644)
	archive.AddFile(file3, "Data\\File3.txt")
	archive.Close()

	archive2, _ := Open(mpqPath)
	defer archive2.Close()

	if !archive2.HasFile("Data\\File1.txt") {
		t.Errorf("original file 1 missing")
	}
	if !archive2.HasFile("Data\\File2.txt") {
		t.Errorf("original file 2 missing")
	}
	if !archive2.HasFile("Data\\File3.txt") {
		t.Errorf("new file 3 missing")
	}

	data, _ := archive2.ReadFile("Data\\File3.txt")
	if string(data) != "New file content" {
		t.Errorf("new file content mismatch: got %q", data)
	}
}

func TestModifyRemoveFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_remove_test_")
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, []byte("Content 1"), 0644)
	os.WriteFile(file2, []byte("Content 2"), 0644)

	mpqPath := filepath.Join(tmpDir, "test.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFile(file1, "Data\\File1.txt")
	archive.AddFile(file2, "Data\\File2.txt")
	archive.Close()

	archive, _ = OpenForModify(mpqPath)
	archive.RemoveFile("Data\\File1.txt")
	archive.Close()

	archive2, _ := Open(mpqPath)
	defer archive2.Close()

	if archive2.HasFile("Data\\File1.txt") {
		t.Errorf("removed file still present")
	}
	if !archive2.HasFile("Data\\File2.txt") {
		t.Errorf("remaining file missing")
	}
}

func TestModifyReplaceFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_replace_test_")
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.txt")
	os.WriteFile(file1, []byte("Original content"), 0644)

	mpqPath := filepath.Join(tmpDir, "test.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFile(file1, "Data\\File.txt")
	archive.Close()

	archive, _ = OpenForModify(mpqPath)
	file1New := filepath.Join(tmpDir, "file1_new.txt")
	os.WriteFile(file1New, []byte("Replaced content"), 0644)
	archive.AddFile(file1New, "Data\\File.txt")
	archive.Close()

	archive2, _ := Open(mpqPath)
	defer archive2.Close()

	data, _ := archive2.ReadFile("Data\\File.txt")
	if string(data) != "Replaced content" {
		t.Errorf("content mismatch: got %q", data)
	}
}

func TestSectorCRC(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_crc_test_")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Test content for CRC"), 0644)

	mpqPath := filepath.Join(tmpDir, "test_crc.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFileWithCRC(testFile, "Data\\Test.txt")
	archive.Close()

	readArchive, _ := Open(mpqPath)
	defer readArchive.Close()

	data, err := readArchive.ReadFile("Data\\Test.txt")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "Test content for CRC" {
		t.Errorf("content mismatch: got %q", data)
	}
}

func TestCRC32Algorithm(t *testing.T) {
	testCases := []struct {
		data     []byte
		expected uint32
	}{
		{[]byte(""), 0x00000000},
		{[]byte("a"), 0xE8B7BE43},
		{[]byte("abc"), 0x352441C2},
		{[]byte("Hello, World!"), 0xEC4AC3D0},
		{[]byte("The quick brown fox jumps over the lazy dog"), 0x414FA339},
	}
	for _, tc := range testCases {
		got := crc32Sum(tc.data)
		if got != tc.expected {
			t.Errorf("CRC32(%q) = 0x%08X, want 0x%08X", tc.data, got, tc.expected)
		}
	}
}

func TestAdler32Algorithm(t *testing.T) {
	testCases := []struct {
		data     []byte
		expected uint32
	}{
		{[]byte(""), 0x00000001},
		{[]byte("a"), 0x00620062},
		{[]byte("abc"), 0x024D0127},
		{[]byte("Wikipedia"), 0x11E60398},
	}
	for _, tc := range testCases {
		got := adler32(tc.data)
		if got != tc.expected {
			t.Errorf("Adler32(%q) = 0x%08X, want 0x%08X", tc.data, got, tc.expected)
		}
	}
}

func TestAddFileFromBytes(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_bytes_test_")
	defer os.RemoveAll(tmpDir)

	mpqPath := filepath.Join(tmpDir, "test.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFileFromBytes([]byte("Hello from bytes"), "Data\\Bytes.txt")
	archive.Close()

	readArchive, _ := Open(mpqPath)
	defer readArchive.Close()

	data, _ := readArchive.ReadFile("Data\\Bytes.txt")
	if string(data) != "Hello from bytes" {
		t.Errorf("content mismatch: got %q", data)
	}
}

func TestGetDBCFiles(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "vanillampq_dbc_test_")
	defer os.RemoveAll(tmpDir)

	mpqPath := filepath.Join(tmpDir, "test.mpq")
	archive, _ := Create(mpqPath, 10)
	archive.AddFileFromBytes([]byte("spell"), "DBFilesClient\\Spell.dbc")
	archive.AddFileFromBytes([]byte("item"), "DBFilesClient\\Item.dbc")
	archive.AddFileFromBytes([]byte("lua"), "Interface\\FrameXML\\Test.lua")
	archive.Close()

	readArchive, _ := Open(mpqPath)
	defer readArchive.Close()

	dbcFiles, _ := readArchive.GetDBCFiles()
	if len(dbcFiles) != 2 {
		t.Errorf("expected 2 DBC files, got %d: %v", len(dbcFiles), dbcFiles)
	}
}

func TestRejectV2Format(t *testing.T) {
	// vanillampq should reject V2 archives
	tmpDir, _ := os.MkdirTemp("", "vanillampq_v2_test_")
	defer os.RemoveAll(tmpDir)

	// Create a file with V2 header manually
	mpqPath := filepath.Join(tmpDir, "v2.mpq")
	f, _ := os.Create(mpqPath)
	h := header{
		Magic:           mpqMagic,
		HeaderSize:      0x2C, // V2
		FormatVersion:   1,    // V2
		SectorSizeShift: 3,
		HashTableSize:   16,
		BlockTableSize:  0,
	}
	writeHeader(f, &h)
	f.Close()

	_, err := Open(mpqPath)
	if err == nil {
		t.Errorf("expected error for V2 archive, got nil")
	}
}
