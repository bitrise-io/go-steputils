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
	// 0 means auto (got default: min(NumCPU*3, 20), minimum 4; OpenStream: 8).
	Concurrency uint

	// MaxRetryPerChunk controls how many times to interrupt and retry a slow chunk.
	// If zero, chunk download monitoring is disabled (OpenStream: no retries).
	MaxRetryPerChunk int

	// ChunkRetryThreshold is the deviation from the moving average
	// after which a chunk is interrupted and retried.
	ChunkRetryThreshold time.Duration

	// HTTPClient is the HTTP client used for download requests.
	// If nil, got's default client is used (OpenStream: a private client with
	// the env transport tuning applied; a non-nil client is used as is).
	HTTPClient *http.Client

	// StreamChunkSize is the Range request size for OpenStream. 0 means 16MB.
	StreamChunkSize int64

	// StreamIdleTimeout cancels and retries an OpenStream chunk attempt that
	// receives no bytes for this long. 0 means 2s.
	StreamIdleTimeout time.Duration

	// StreamHedgeDelay starts a second, racing request for the chunk Read is
	// blocked on once it has waited this long (or 3x its average wait so far, if
	// that is longer); the first success wins. It catches chunks that trickle in
	// too slowly to trip StreamIdleTimeout. 0 means 1s, negative disables hedging.
	StreamHedgeDelay time.Duration
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
