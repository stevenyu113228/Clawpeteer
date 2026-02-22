package filetransfer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestReceiveChunksInOrder(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewReceiver(tmpDir)

	content := []byte("Hello, World! This is a test file for chunked transfer.")
	expectedHash := sha256Hash(content)

	chunkSize := 16
	chunks := splitIntoChunks(content, chunkSize)

	destPath := filepath.Join(tmpDir, "output", "test.txt")

	err := r.InitTransfer("txn-1", "test.txt", destPath, int64(len(content)), len(chunks), expectedHash)
	if err != nil {
		t.Fatalf("InitTransfer: %v", err)
	}

	// Send chunks in order
	for i, chunk := range chunks {
		encoded := base64.StdEncoding.EncodeToString(chunk)
		if err := r.ReceiveChunk("txn-1", i, encoded); err != nil {
			t.Fatalf("ReceiveChunk(%d): %v", i, err)
		}
	}

	verified, err := r.Finalize("txn-1")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if !verified {
		t.Fatal("Checksum verification failed")
	}

	// Verify file content
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("Content mismatch: got %q, want %q", string(got), string(content))
	}
}

func TestReceiveChunksOutOfOrder(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewReceiver(tmpDir)

	content := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	expectedHash := sha256Hash(content)

	chunkSize := 10
	chunks := splitIntoChunks(content, chunkSize)

	destPath := filepath.Join(tmpDir, "output", "ooo.txt")

	err := r.InitTransfer("txn-ooo", "ooo.txt", destPath, int64(len(content)), len(chunks), expectedHash)
	if err != nil {
		t.Fatalf("InitTransfer: %v", err)
	}

	// Send chunks out of order: last, first, middle
	order := []int{len(chunks) - 1, 0}
	for i := 1; i < len(chunks)-1; i++ {
		order = append(order, i)
	}

	for _, idx := range order {
		encoded := base64.StdEncoding.EncodeToString(chunks[idx])
		if err := r.ReceiveChunk("txn-ooo", idx, encoded); err != nil {
			t.Fatalf("ReceiveChunk(%d): %v", idx, err)
		}
	}

	verified, err := r.Finalize("txn-ooo")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if !verified {
		t.Fatal("Checksum verification failed for out-of-order chunks")
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("Content mismatch: got %q, want %q", string(got), string(content))
	}
}

func TestMissingChunks(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewReceiver(tmpDir)

	destPath := filepath.Join(tmpDir, "output", "missing.txt")

	err := r.InitTransfer("txn-miss", "missing.txt", destPath, 50, 5, "abc123")
	if err != nil {
		t.Fatalf("InitTransfer: %v", err)
	}

	// Send chunks 0, 2, 4 (skip 1 and 3)
	for _, idx := range []int{0, 2, 4} {
		data := base64.StdEncoding.EncodeToString([]byte("chunk data"))
		if err := r.ReceiveChunk("txn-miss", idx, data); err != nil {
			t.Fatalf("ReceiveChunk(%d): %v", idx, err)
		}
	}

	missing, err := r.MissingChunks("txn-miss")
	if err != nil {
		t.Fatalf("MissingChunks: %v", err)
	}

	if len(missing) != 2 {
		t.Fatalf("Expected 2 missing chunks, got %d", len(missing))
	}
	if missing[0] != 1 || missing[1] != 3 {
		t.Fatalf("Expected missing chunks [1, 3], got %v", missing)
	}

	// Verify progress
	received, total, ok := r.Progress("txn-miss")
	if !ok {
		t.Fatal("Progress returned not found")
	}
	if received != 3 || total != 5 {
		t.Fatalf("Expected progress 3/5, got %d/%d", received, total)
	}
}

func TestPrepareDownload(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte("Download me! This is a file to be downloaded via chunked transfer.")
	filePath := filepath.Join(tmpDir, "download.txt")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	chunkSize := 20
	meta, err := PrepareDownload(filePath, chunkSize)
	if err != nil {
		t.Fatalf("PrepareDownload: %v", err)
	}

	if meta.Filename != "download.txt" {
		t.Fatalf("Expected filename 'download.txt', got %q", meta.Filename)
	}
	if meta.Size != int64(len(content)) {
		t.Fatalf("Expected size %d, got %d", len(content), meta.Size)
	}

	expectedChunks := len(content) / chunkSize
	if len(content)%chunkSize != 0 {
		expectedChunks++
	}
	if meta.TotalChunks != expectedChunks {
		t.Fatalf("Expected %d chunks, got %d", expectedChunks, meta.TotalChunks)
	}

	expectedHash := sha256Hash(content)
	if meta.Sha256 != expectedHash {
		t.Fatalf("Expected sha256 %s, got %s", expectedHash, meta.Sha256)
	}

	// Verify ReadChunk works for each chunk
	var reassembled []byte
	for i := 0; i < meta.TotalChunks; i++ {
		encoded, err := ReadChunk(filePath, i, chunkSize)
		if err != nil {
			t.Fatalf("ReadChunk(%d): %v", i, err)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("base64 decode chunk %d: %v", i, err)
		}
		reassembled = append(reassembled, decoded...)
	}

	if string(reassembled) != string(content) {
		t.Fatalf("Reassembled content mismatch")
	}
}

// Helper: compute SHA-256 hex string
func sha256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Helper: split data into chunks of the given size
func splitIntoChunks(data []byte, chunkSize int) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}
	return chunks
}
