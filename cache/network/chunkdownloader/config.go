package chunkdownloader

import (
	"net/http"
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the chunk downloader.
type Config struct {
	// Concurrency is the number of parallel chunk downloads.
	// 0 means auto (got default: min(NumCPU*3, 20), minimum 4).
	Concurrency uint

	// MaxRetryPerChunk controls how many times to interrupt and retry a slow chunk.
	// If zero, chunk download monitoring is disabled.
	MaxRetryPerChunk int

	// ChunkRetryThreshold is the deviation from the moving average
	// after which a chunk is interrupted and retried.
	ChunkRetryThreshold time.Duration

	// HTTPClient is the HTTP client used for download requests.
	// If nil, got's default client is used.
	HTTPClient *http.Client
}

// DefaultConfig returns the default configuration.
// Values can be overridden by BITRISEIO_DEPENDENCY_CACHE_* environment variables.
func DefaultConfig() Config {
	cfg := Config{
		Concurrency:         0,
		MaxRetryPerChunk:    5,
		ChunkRetryThreshold: 10 * time.Second,
	}

	if val, err := strconv.Atoi(os.Getenv("BITRISEIO_DEPENDENCY_CACHE_MAX_RETRY_PER_CHUNK")); err == nil {
		cfg.MaxRetryPerChunk = val
	}

	if val, err := strconv.Atoi(os.Getenv("BITRISEIO_DEPENDENCY_CACHE_CHUNK_RETRY_THRESHOLD")); err == nil {
		cfg.ChunkRetryThreshold = time.Duration(val) * time.Second
	}

	return cfg
}
