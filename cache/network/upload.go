package network

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
)

// DefaultUploader ...
type DefaultUploader struct{}

// chunkStatistics tracks upload performance for retrying
type chunkStatistics struct {
	sum            time.Duration
	finishedChunks int64
	mu             sync.Mutex
}

func (cs *chunkStatistics) update(d time.Duration) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.sum += d
	cs.finishedChunks++
}

func (cs *chunkStatistics) average() time.Duration {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.finishedChunks == 0 {
		return 0
	}
	return cs.sum / time.Duration(cs.finishedChunks)
}

func (cs *chunkStatistics) getFinishedCount() int64 {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.finishedChunks
}

// UploadParams ...
type UploadParams struct {
	APIBaseURL      string
	Token           string
	ArchivePath     string
	ArchiveChecksum string
	ArchiveSize     int64
	CacheKey        string
}

// Upload a cache archive and associate it with the provided cache key
func (u DefaultUploader) Upload(ctx context.Context, params UploadParams, logger log.Logger) error {
	validatedKey, err := validateKey(params.CacheKey, logger)
	if err != nil {
		return err
	}

	client := newAPIClient(retryhttp.NewClient(logger), params.APIBaseURL, params.Token, logger)

	optimalChunkSizeMB := int(getDefaultChunkSizeBytes(
		uint64(params.ArchiveSize), 8*1024*1024,
		100*1024*1024,
		uint64(getDefaultConcurrency())) / 1024 / 1024)

	logger.Debugf("Using multipart upload for file (%d bytes) with chunk size %d MB", params.ArchiveSize, optimalChunkSizeMB)
	logger.Debugf("Calculated chunk size: %d MB for file size: %d bytes (%d MB)", optimalChunkSizeMB, params.ArchiveSize, params.ArchiveSize/(1024*1024))
	return u.uploadWithMultipart(ctx, params, validatedKey, client, logger, optimalChunkSizeMB)
}

func (u DefaultUploader) uploadWithMultipart(ctx context.Context, params UploadParams, validatedKey string, client apiClient, logger log.Logger, chunkSizeMB int) error {
	logger.Debugf("Prepare multipart upload")
	prepareUploadRequest := prepareUploadRequest{
		CacheKey:           validatedKey,
		ArchiveFileName:    filepath.Base(params.ArchivePath),
		ArchiveContentType: "application/zstd",
		ArchiveSizeInBytes: params.ArchiveSize,
		ChunkSizeMB:        chunkSizeMB,
	}

	multipartResp, err := client.prepareMultipartUpload(prepareUploadRequest)
	if err != nil {
		return fmt.Errorf("failed to prepare multipart upload: %w", err)
	}

	logger.Debugf("Multipart Upload ID: %s", multipartResp.ID)
	logger.Debugf("Chunk count: %d, Chunk size: %d bytes", multipartResp.ChunkCount, multipartResp.ChunkSizeBytes)

	logger.Debugf("Upload chunks")
	etags, err := u.uploadChunks(ctx, params.ArchivePath, multipartResp, logger)
	if err != nil {
		return fmt.Errorf("failed to upload chunks: %w", err)
	}

	logger.Debugf("Complete multipart upload")
	response, err := client.completeMultipartUpload(multipartResp.ID, etags)
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	logger.Debugf("Multipart upload completed")
	logResponseMessage(response, logger)

	return nil
}

func (u DefaultUploader) uploadChunks(ctx context.Context, archivePath string, response prepareMultipartUploadResponse, logger log.Logger) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Errorf("close archive: %v", err)
		}
	}()

	chunkSize := response.ChunkSizeBytes
	numChunks := len(response.URLs)

	chunks := make([][]byte, numChunks)
	currentOffset := int64(0)

	for i := 0; i < numChunks; i++ {
		currentChunkSize := chunkSize
		if i == numChunks-1 {
			currentChunkSize = response.LastChunkSizeBytes
		}

		// seek to correct position
		_, err := file.Seek(currentOffset, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to position %d for chunk %d: %w", currentOffset, i+1, err)
		}

		// read chunk from file
		chunk := make([]byte, currentChunkSize)
		n, err := io.ReadFull(file, chunk)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read chunk %d: %w", i+1, err)
		}

		if n == 0 {
			return nil, fmt.Errorf("unexpected end of file at chunk %d", i+1)
		}

		chunks[i] = chunk[:n]
		currentOffset += int64(n)
	}

	// MaxRetryPerChunk is controls how many times to interrupt and retry a slow chunk upload.
	// If zero, the chunk download monitoring is disabled and the chunk download won't be interrupted.
	maxRetryPerChunk := 3

	// ChunkRetryThreshold is the deviation from the moving average (of chunks uploaded so far) after which a chunk is interrupted and retried.
	chunkRetryThreshold := 10 * time.Second

	var stats chunkStatistics

	type chunkResult struct {
		index int
		etag  string
		err   error
	}

	resultChan := make(chan chunkResult, numChunks)
	semaphore := make(chan struct{}, getDefaultConcurrency())

	for i, uploadURL := range response.URLs {
		go func(index int, url prepareMultipartUploadURL, chunkData []byte) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var etag string
			var uploadErr error

			for attempt := 0; attempt < maxRetryPerChunk; attempt++ {
				logger.Debugf("Uploading chunk %d/%d (attempt %d/%d) [finished=%d] [avg=%v]",
					index+1, numChunks, attempt+1, maxRetryPerChunk,
					stats.getFinishedCount(), stats.average().Round(time.Second))

				start := time.Now()

				chunkCtx, cancelChunk := context.WithCancel(ctx)

				if attempt < maxRetryPerChunk-1 {
					go func() {
						ticker := time.NewTicker(time.Second)
						defer ticker.Stop()

						for {
							select {
							case <-chunkCtx.Done():
								return
							case <-ticker.C:
								if stats.getFinishedCount() > 0 && time.Since(start)-stats.average() > chunkRetryThreshold {
									logger.Warnf("⚠️ found hanged chunk upload, canceling request after %s",
										time.Since(start).Round(time.Second))
									cancelChunk()
									return
								}
							}
						}
					}()
				}

				etag, uploadErr = u.uploadChunkWithContext(chunkCtx, url.Method, url.URL, url.Headers, chunkData, logger)
				cancelChunk()

				if uploadErr == nil {
					took := time.Since(start)
					stats.update(took)
					logger.Infof("Chunk %d uploaded successfully in %v, ETag: %s",
						index+1, took.Round(time.Second), etag)
					break
				}

				if chunkCtx.Err() == context.Canceled {
					logger.Warnf("Chunk %d attempt %d cancelled due to slow speed, retrying...", index+1, attempt+1)
					continue
				}

				logger.Warnf("Chunk %d attempt %d failed: %v", index+1, attempt+1, uploadErr)

				if attempt == maxRetryPerChunk-1 {
					break
				}
			}

			resultChan <- chunkResult{
				index: index,
				etag:  etag,
				err:   uploadErr,
			}
		}(i, uploadURL, chunks[i])
	}

	etags := make([]string, numChunks)
	for i := 0; i < numChunks; i++ {
		result := <-resultChan
		if result.err != nil {
			return nil, fmt.Errorf("failed to upload chunk %d after %d attempts: %w", result.index+1, maxRetryPerChunk, result.err)
		}
		etags[result.index] = result.etag
	}

	return etags, nil
}

func (u DefaultUploader) uploadChunkWithContext(ctx context.Context, method, url string, headers map[string]string, chunk []byte, logger log.Logger) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(chunk))
	if err != nil {
		return "", fmt.Errorf("failed to create chunk upload request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(chunk)))

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("chunk upload cancelled: %w", ctx.Err())
		}
		return "", fmt.Errorf("failed to upload chunk: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Errorf("close response body: %v", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chunk upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		return "", fmt.Errorf("no ETag returned from chunk upload")
	}

	// TODO do i need this?
	// Remove quotes from ETag if present
	etagRegex := regexp.MustCompile(`^"?([^"]*)"?$`)
	matches := etagRegex.FindStringSubmatch(etag)
	if len(matches) > 1 {
		etag = matches[1]
	}

	return etag, nil
}

func getDefaultChunkSizeBytes(totalSize, min, max, concurrency uint64) uint64 {

	cs := totalSize / concurrency

	// if chunk size >= 102400000 bytes set default to (ChunkSize / 2)
	if cs >= 102400000 {
		cs = cs / 2
	}

	// Set default min chunk size to 2m, or file size / 2
	if min == 0 {

		min = 2097152

		if min >= totalSize {
			min = totalSize / 2
		}
	}

	// if Chunk size < Min size set chunk size to min.
	if cs < min {
		cs = min
	}

	// Change ChunkSize if MaxChunkSize are set and ChunkSize > Max size
	if max > 0 && cs > max {
		cs = max
	}

	// When chunk size > total file size, divide chunk / 2
	if cs >= totalSize {
		cs = totalSize / 2
	}

	return cs
}

func getDefaultConcurrency() uint {
	c := uint(runtime.NumCPU() * 3)

	// Set default max concurrency to 20.
	if c > 20 {
		c = 20
	}

	// Set default min concurrency to 4.
	if c <= 2 {
		c = 4
	}

	return c
}

func validateKey(key string, logger log.Logger) (string, error) {
	if strings.Contains(key, ",") {
		return "", fmt.Errorf("commas are not allowed in key")
	}

	if len(key) > maxKeyLength {
		logger.Warnf("Key is too long, truncating it to the first %d characters", maxKeyLength)
		return key[:maxKeyLength], nil
	}
	return key, nil
}

func logResponseMessage(response acknowledgeResponse, logger log.Logger) {
	if response.Message == "" || response.Severity == "" {
		return
	}

	var loggerFn func(format string, v ...interface{})
	switch response.Severity {
	case "debug":
		loggerFn = logger.Debugf
	case "info":
		loggerFn = logger.Infof
	case "warning":
		loggerFn = logger.Warnf
	case "error":
		loggerFn = logger.Errorf
	default:
		loggerFn = logger.Printf
	}

	loggerFn("\n")
	loggerFn(response.Message)
	loggerFn("\n")
}
