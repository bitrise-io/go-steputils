package chunkuploader

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
)

// FileChunkProvider reads chunks from a file on disk.
// Thread-safe for parallel chunk reads.
type FileChunkProvider struct {
	file          *os.File
	chunkSize     int64
	lastChunkSize int64
	numChunks     int
	mu            sync.Mutex
}

// NewFileChunkProvider creates a ChunkProvider that reads from a file.
func NewFileChunkProvider(path string, chunkSize, lastChunkSize int64, numChunks int) (*FileChunkProvider, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	return &FileChunkProvider{
		file:          file,
		chunkSize:     chunkSize,
		lastChunkSize: lastChunkSize,
		numChunks:     numChunks,
	}, nil
}

// NumChunks returns the total number of chunks.
func (p *FileChunkProvider) NumChunks() int {
	return p.numChunks
}

// ChunkSize returns the size of the chunk at the given index.
func (p *FileChunkProvider) ChunkSize(index int) int64 {
	if index == p.numChunks-1 {
		return p.lastChunkSize
	}
	return p.chunkSize
}

// GetChunk returns a reader for the chunk at the given index.
// The data is read into memory to allow for retries.
func (p *FileChunkProvider) GetChunk(index int) (io.Reader, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	size := p.ChunkSize(index)
	offset := int64(index) * p.chunkSize

	_, err := p.file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("seek to position %d for chunk %d: %w", offset, index+1, err)
	}

	chunk := make([]byte, size)
	n, err := io.ReadFull(p.file, chunk)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("read chunk %d: %w", index+1, err)
	}

	if n == 0 {
		return nil, fmt.Errorf("unexpected end of file at chunk %d", index+1)
	}

	return bytes.NewReader(chunk[:n]), nil
}

// Close closes the underlying file.
func (p *FileChunkProvider) Close() error {
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}

// ByteSliceChunkProvider provides chunks from pre-loaded byte slices.
// Useful for streaming scenarios where data is already in memory.
type ByteSliceChunkProvider struct {
	chunks [][]byte
}

// NewByteSliceChunkProvider creates a ChunkProvider from byte slices.
func NewByteSliceChunkProvider(chunks [][]byte) *ByteSliceChunkProvider {
	return &ByteSliceChunkProvider{chunks: chunks}
}

// NumChunks returns the total number of chunks.
func (p *ByteSliceChunkProvider) NumChunks() int {
	return len(p.chunks)
}

// ChunkSize returns the size of the chunk at the given index.
func (p *ByteSliceChunkProvider) ChunkSize(index int) int64 {
	if index < 0 || index >= len(p.chunks) {
		return 0
	}
	return int64(len(p.chunks[index]))
}

// GetChunk returns a reader for the chunk at the given index.
func (p *ByteSliceChunkProvider) GetChunk(index int) (io.Reader, error) {
	if index < 0 || index >= len(p.chunks) {
		return nil, fmt.Errorf("chunk index %d out of range [0, %d)", index, len(p.chunks))
	}
	return bytes.NewReader(p.chunks[index]), nil
}

// StreamChunkProvider accumulates chunks from a stream and provides them for upload.
// Useful for proxy scenarios where data arrives as a stream.
type StreamChunkProvider struct {
	chunks    [][]byte
	chunkSize int64
	mu        sync.RWMutex
}

// NewStreamChunkProvider creates a ChunkProvider for streaming data.
func NewStreamChunkProvider(expectedChunks int, chunkSize int64) *StreamChunkProvider {
	return &StreamChunkProvider{
		chunks:    make([][]byte, expectedChunks),
		chunkSize: chunkSize,
	}
}

// AddChunk adds a chunk at the specified index.
// Thread-safe for concurrent chunk additions.
func (p *StreamChunkProvider) AddChunk(index int, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index < 0 || index >= len(p.chunks) {
		return fmt.Errorf("chunk index %d out of range [0, %d)", index, len(p.chunks))
	}

	// Make a copy of the data to avoid issues with buffer reuse
	p.chunks[index] = make([]byte, len(data))
	copy(p.chunks[index], data)

	return nil
}

// NumChunks returns the total number of chunks.
func (p *StreamChunkProvider) NumChunks() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.chunks)
}

// ChunkSize returns the size of the chunk at the given index.
func (p *StreamChunkProvider) ChunkSize(index int) int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if index < 0 || index >= len(p.chunks) || p.chunks[index] == nil {
		return 0
	}
	return int64(len(p.chunks[index]))
}

// GetChunk returns a reader for the chunk at the given index.
func (p *StreamChunkProvider) GetChunk(index int) (io.Reader, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if index < 0 || index >= len(p.chunks) {
		return nil, fmt.Errorf("chunk index %d out of range [0, %d)", index, len(p.chunks))
	}

	if p.chunks[index] == nil {
		return nil, fmt.Errorf("chunk %d not yet received", index)
	}

	return bytes.NewReader(p.chunks[index]), nil
}

// IsComplete returns true if all chunks have been received.
func (p *StreamChunkProvider) IsComplete() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, chunk := range p.chunks {
		if chunk == nil {
			return false
		}
	}
	return true
}

// ReceivedCount returns the number of chunks received so far.
func (p *StreamChunkProvider) ReceivedCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, chunk := range p.chunks {
		if chunk != nil {
			count++
		}
	}
	return count
}
