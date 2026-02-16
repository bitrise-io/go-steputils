// Package chunkuploader provides a reusable, optimized chunk upload system for R2/S3 storage.
// It supports parallel uploads, hung request detection, and automatic retries.
package chunkuploader

import (
	"io"
)

// UploadURL represents a signed URL for uploading a single chunk.
type UploadURL struct {
	Method  string
	URL     string
	Headers map[string]string
}

// ChunkProvider provides chunk data for upload.
// Implementations can read from files, memory buffers, or streams.
type ChunkProvider interface {
	// NumChunks returns the total number of chunks.
	NumChunks() int

	// ChunkSize returns the size of the chunk at the given index.
	ChunkSize(index int) int64

	// GetChunk returns a reader for the chunk at the given index.
	// The reader should be valid for the lifetime of the upload attempt.
	// For retries, GetChunk may be called multiple times for the same index.
	GetChunk(index int) (io.Reader, error)
}

// ChunkResult represents the result of uploading a single chunk.
type ChunkResult struct {
	Index int
	ETag  string
	Err   error
}

// UploadResult represents the result of uploading all chunks.
type UploadResult struct {
	ETags []string
}
