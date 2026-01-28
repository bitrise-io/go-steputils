package chunkuploader

import (
	"net/http"
	"runtime"
	"time"
)

// Config holds configuration for the chunk uploader.
type Config struct {
	// Concurrency is the maximum number of parallel chunk uploads.
	// Default: min(NumCPU * 3, 20), minimum 2
	Concurrency int

	// MaxRetryPerChunk is the maximum number of retry attempts per chunk.
	// Default: 3
	MaxRetryPerChunk int

	// HungThreshold is the duration after which a chunk upload is considered hung
	// if it exceeds the average upload time by this amount.
	// Default: 30 seconds
	HungThreshold time.Duration

	// HTTPClient is the HTTP client to use for uploads.
	// If nil, a default optimized client will be created.
	HTTPClient *http.Client
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Concurrency:      DefaultConcurrency(),
		MaxRetryPerChunk: 3,
		HungThreshold:    30 * time.Second,
		HTTPClient:       nil, // Will be created by Uploader
	}
}

// DefaultConcurrency calculates the default concurrency based on CPU count.
func DefaultConcurrency() int {
	c := runtime.NumCPU() * 3

	if c > 20 {
		c = 20
	}

	if c < 2 {
		c = 2
	}

	return c
}

// DefaultHTTPClient creates an HTTP client optimized for chunk uploads.
func DefaultHTTPClient() *http.Client {
	return &http.Client{
		// No timeout - individual chunk timeouts are handled via context
		Timeout: 0,
		Transport: &http.Transport{
			MaxIdleConns:        50,
			MaxConnsPerHost:     20,
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
			Proxy:               http.ProxyFromEnvironment,
		},
	}
}

// OptimalChunkSizeBytes calculates optimal chunk size based on total size and concurrency.
func OptimalChunkSizeBytes(totalSize int64, concurrency int) int64 {
	return int64(optimalChunkSizeBytes(uint64(totalSize), 8*1024*1024, 100*1024*1024, uint64(concurrency)))
}

func optimalChunkSizeBytes(totalSize, min, max, concurrency uint64) uint64 {
	cs := totalSize / concurrency

	// Reduce chunk size for very large chunks to improve parallelism
	if cs >= 100*1024*1024 {
		cs = cs / 2
	}

	if cs < min {
		cs = min
	}

	if max > 0 && cs > max {
		cs = max
	}

	return cs
}
