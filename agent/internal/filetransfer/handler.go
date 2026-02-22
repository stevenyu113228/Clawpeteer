package filetransfer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Transfer tracks the state of a single file transfer.
type Transfer struct {
	ID             string
	Filename       string
	DestPath       string
	Size           int64
	TotalChunks    int
	ExpectedSha256 string
	ReceivedChunks map[int]bool
	ChunkDir       string
}

// Receiver manages incoming file transfers with chunked upload support.
type Receiver struct {
	baseDir   string
	transfers map[string]*Transfer
	mu        sync.Mutex
}

// NewReceiver creates a new Receiver that stores temporary chunks under baseDir.
func NewReceiver(baseDir string) *Receiver {
	return &Receiver{
		baseDir:   baseDir,
		transfers: make(map[string]*Transfer),
	}
}

// InitTransfer creates a chunk directory and registers a new transfer.
func (r *Receiver) InitTransfer(id, filename, destPath string, size int64, totalChunks int, sha256sum string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transfers[id]; exists {
		return fmt.Errorf("transfer %s already exists", id)
	}

	chunkDir := filepath.Join(r.baseDir, "chunks", id)
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		return fmt.Errorf("create chunk dir: %w", err)
	}

	r.transfers[id] = &Transfer{
		ID:             id,
		Filename:       filename,
		DestPath:       destPath,
		Size:           size,
		TotalChunks:    totalChunks,
		ExpectedSha256: sha256sum,
		ReceivedChunks: make(map[int]bool),
		ChunkDir:       chunkDir,
	}

	return nil
}

// ReceiveChunk decodes base64 data and writes it to a chunk file.
func (r *Receiver) ReceiveChunk(id string, index int, data string) error {
	r.mu.Lock()
	t, ok := r.transfers[id]
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}

	if index < 0 || index >= t.TotalChunks {
		return fmt.Errorf("chunk index %d out of range [0, %d)", index, t.TotalChunks)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}

	chunkPath := filepath.Join(t.ChunkDir, fmt.Sprintf("chunk_%06d", index))
	if err := os.WriteFile(chunkPath, decoded, 0o644); err != nil {
		return fmt.Errorf("write chunk: %w", err)
	}

	r.mu.Lock()
	t.ReceivedChunks[index] = true
	r.mu.Unlock()

	return nil
}

// Progress returns the number of received and total chunks for a transfer.
func (r *Receiver) Progress(id string) (received, total int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, exists := r.transfers[id]
	if !exists {
		return 0, 0, false
	}

	return len(t.ReceivedChunks), t.TotalChunks, true
}

// MissingChunks returns the indices of chunks that have not been received.
func (r *Receiver) MissingChunks(id string) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.transfers[id]
	if !ok {
		return nil, fmt.Errorf("transfer %s not found", id)
	}

	var missing []int
	for i := 0; i < t.TotalChunks; i++ {
		if !t.ReceivedChunks[i] {
			missing = append(missing, i)
		}
	}
	return missing, nil
}

// Finalize assembles all chunks into the destination file, verifies the
// SHA-256 checksum, and cleans up chunk files. Returns whether the
// checksum was verified successfully.
func (r *Receiver) Finalize(id string) (bool, error) {
	r.mu.Lock()
	t, ok := r.transfers[id]
	r.mu.Unlock()

	if !ok {
		return false, fmt.Errorf("transfer %s not found", id)
	}

	// Verify all chunks received
	if len(t.ReceivedChunks) != t.TotalChunks {
		return false, fmt.Errorf("missing chunks: received %d of %d", len(t.ReceivedChunks), t.TotalChunks)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(t.DestPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return false, fmt.Errorf("create dest dir: %w", err)
	}

	// Create destination file
	destFile, err := os.Create(t.DestPath)
	if err != nil {
		return false, fmt.Errorf("create dest file: %w", err)
	}
	defer destFile.Close()

	// Assemble chunks in order
	hasher := sha256.New()
	writer := io.MultiWriter(destFile, hasher)

	// Get sorted chunk indices
	indices := make([]int, 0, t.TotalChunks)
	for i := range t.ReceivedChunks {
		indices = append(indices, i)
	}
	sort.Ints(indices)

	for _, idx := range indices {
		chunkPath := filepath.Join(t.ChunkDir, fmt.Sprintf("chunk_%06d", idx))
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			return false, fmt.Errorf("read chunk %d: %w", idx, err)
		}
		if _, err := writer.Write(chunkData); err != nil {
			return false, fmt.Errorf("write chunk %d: %w", idx, err)
		}
	}

	// Calculate and verify checksum
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	verified := t.ExpectedSha256 == "" || actualHash == t.ExpectedSha256

	// Clean up chunk directory
	os.RemoveAll(t.ChunkDir)

	// Remove transfer from map
	r.mu.Lock()
	delete(r.transfers, id)
	r.mu.Unlock()

	return verified, nil
}

// DownloadMeta holds metadata for a file download.
type DownloadMeta struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	TotalChunks int    `json:"totalChunks"`
	Sha256      string `json:"sha256"`
}

// PrepareDownload reads a file, calculates its SHA-256, and returns metadata.
func PrepareDownload(filePath string, chunkSize int) (*DownloadMeta, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, fmt.Errorf("hash file: %w", err)
	}

	size := info.Size()
	totalChunks := int(size) / chunkSize
	if int(size)%chunkSize != 0 {
		totalChunks++
	}
	if totalChunks == 0 {
		totalChunks = 1
	}

	return &DownloadMeta{
		Filename:    filepath.Base(filePath),
		Size:        size,
		TotalChunks: totalChunks,
		Sha256:      hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

// ReadChunk reads a specific chunk from a file and returns base64-encoded data.
func ReadChunk(filePath string, index, chunkSize int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	offset := int64(index) * int64(chunkSize)
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek: %w", err)
	}

	buf := make([]byte, chunkSize)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read chunk: %w", err)
	}
	if n == 0 {
		return "", fmt.Errorf("chunk %d is empty", index)
	}

	return base64.StdEncoding.EncodeToString(buf[:n]), nil
}
