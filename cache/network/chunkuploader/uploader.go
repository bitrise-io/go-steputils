package chunkuploader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
)

// Uploader handles parallel chunk uploads with retry and hung detection.
type Uploader struct {
	config     Config
	httpClient *http.Client
	logger     log.Logger
	stats      *Stats
}

// New creates a new Uploader with the given configuration.
func New(config Config) *Uploader {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = DefaultHTTPClient()
	}

	return &Uploader{
		config:     config,
		httpClient: httpClient,
		logger:     log.NewLogger(),
		stats:      NewStats(),
	}
}

// Upload uploads all chunks from the provider to the given URLs in parallel.
// Returns the ETags in the same order as the URLs.
func (u *Uploader) Upload(ctx context.Context, provider ChunkProvider, urls []UploadURL) (*UploadResult, error) {
	numChunks := provider.NumChunks()
	if numChunks != len(urls) {
		return nil, fmt.Errorf("chunk count mismatch: provider has %d chunks, but %d URLs provided", numChunks, len(urls))
	}

	if numChunks == 0 {
		return &UploadResult{ETags: []string{}}, nil
	}

	resultChan := make(chan ChunkResult, numChunks)
	semaphore := make(chan struct{}, u.config.Concurrency)

	// Launch parallel uploads
	for i := 0; i < numChunks; i++ {
		go func(index int, url UploadURL) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			etag, err := u.uploadChunkWithRetry(ctx, provider, url, index, numChunks)
			resultChan <- ChunkResult{
				Index: index,
				ETag:  etag,
				Err:   err,
			}
		}(i, urls[i])
	}

	// Collect results
	etags := make([]string, numChunks)
	completedChunks := 0
	for completedChunks < numChunks {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("upload cancelled while waiting for chunks: %w", ctx.Err())
		case result := <-resultChan:
			completedChunks++
			if result.Err != nil {
				return nil, fmt.Errorf("chunk %d failed after %d attempts: %w",
					result.Index+1, u.config.MaxRetryPerChunk, result.Err)
			}
			etags[result.Index] = result.ETag
		}
	}

	return &UploadResult{ETags: etags}, nil
}

// UploadSingleChunk uploads a single chunk with retry logic.
// Useful for streaming scenarios where chunks arrive one at a time.
func (u *Uploader) UploadSingleChunk(ctx context.Context, data []byte, url UploadURL, index, totalChunks int) (string, error) {
	provider := &singleChunkProvider{data: data}
	return u.uploadChunkWithRetry(ctx, provider, url, 0, totalChunks)
}

// Stats returns the upload statistics.
func (u *Uploader) Stats() *Stats {
	return u.stats
}

// CloseIdleConnections closes idle connections in the HTTP client.
func (u *Uploader) CloseIdleConnections() {
	if transport, ok := u.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

func (u *Uploader) uploadChunkWithRetry(ctx context.Context, provider ChunkProvider, url UploadURL, index, totalChunks int) (string, error) {
	var etag string
	var uploadErr error

	for attempt := 0; attempt < u.config.MaxRetryPerChunk; attempt++ {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("chunk %d upload cancelled: %w", index+1, ctx.Err())
		default:
		}

		u.logger.Debugf("Uploading chunk %d/%d (attempt %d/%d) [finished=%d] [avg=%v]",
			index+1, totalChunks, attempt+1, u.config.MaxRetryPerChunk,
			u.stats.FinishedCount(), u.stats.Average().Round(time.Second))

		start := time.Now()

		chunkCtx, cancelChunk := context.WithCancel(ctx)

		// Start hung detection goroutine (except on last retry)
		if attempt < u.config.MaxRetryPerChunk-1 && u.config.HungThreshold > 0 {
			go u.detectHungUpload(chunkCtx, cancelChunk, start, index)
		}

		etag, uploadErr = u.uploadChunk(chunkCtx, provider, url, index)
		cancelChunk()

		if uploadErr == nil {
			took := time.Since(start)
			u.stats.Update(took)
			u.logger.Infof("Chunk %d uploaded successfully in %v, ETag: %s",
				index+1, took.Round(time.Second), etag)
			return etag, nil
		}

		u.logger.Warnf("Chunk %d attempt %d failed: %v", index+1, attempt+1, uploadErr)

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("chunk %d upload cancelled: %w", index+1, ctx.Err())
		default:
			if chunkCtx.Err() == context.Canceled {
				// Hung detection cancelled this request, retry with backoff
				backoff := time.Duration((attempt+1)*2) * time.Second
				u.logger.Warnf("Chunk %d attempt %d cancelled (hung), retrying after %v", index+1, attempt+1, backoff)
				time.Sleep(backoff)
				continue
			}
		}
	}

	return "", fmt.Errorf("upload chunk %d: %w", index+1, uploadErr)
}

func (u *Uploader) detectHungUpload(ctx context.Context, cancel context.CancelFunc, start time.Time, index int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if u.stats.FinishedCount() > 0 {
				elapsed := time.Since(start)
				avg := u.stats.Average()
				if elapsed-avg > u.config.HungThreshold {
					u.logger.Warnf("Found hung chunk upload (chunk %d); canceling request after %s (avg: %s)",
						index+1, elapsed.Round(time.Second), avg.Round(time.Second))
					cancel()
					return
				}
			}
		}
	}
}

func (u *Uploader) uploadChunk(ctx context.Context, provider ChunkProvider, url UploadURL, index int) (string, error) {
	reader, err := provider.GetChunk(index)
	if err != nil {
		return "", fmt.Errorf("get chunk %d: %w", index+1, err)
	}

	chunkSize := provider.ChunkSize(index)

	// If the reader is a bytes.Reader, we can reuse it for retries
	// Otherwise, we need to read it into memory for potential retries
	var body io.Reader = reader
	if _, ok := reader.(*bytes.Reader); !ok {
		// Read into memory for retry support
		data, err := io.ReadAll(reader)
		if err != nil {
			return "", fmt.Errorf("read chunk %d: %w", index+1, err)
		}
		body = bytes.NewReader(data)
		chunkSize = int64(len(data))
	}

	req, err := http.NewRequestWithContext(ctx, url.Method, url.URL, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	for k, v := range url.Headers {
		req.Header.Set(k, v)
	}
	req.ContentLength = chunkSize

	resp, err := u.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("chunk upload cancelled: %w", ctx.Err())
		}
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorBody := make([]byte, 1024)
		n, _ := io.ReadAtLeast(resp.Body, errorBody, 1)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(errorBody[:n]))
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		return "", fmt.Errorf("no ETag in response")
	}

	return etag, nil
}

// singleChunkProvider wraps a byte slice as a ChunkProvider for single-chunk uploads.
type singleChunkProvider struct {
	data []byte
}

func (p *singleChunkProvider) NumChunks() int {
	return 1
}

func (p *singleChunkProvider) ChunkSize(index int) int64 {
	return int64(len(p.data))
}

func (p *singleChunkProvider) GetChunk(index int) (io.Reader, error) {
	return bytes.NewReader(p.data), nil
}
