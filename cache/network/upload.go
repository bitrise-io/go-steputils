package network

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
		return fmt.Errorf("validating cache key: %w", err)
	}

	client := newAPIClient(retryhttp.NewClient(logger), params.APIBaseURL, params.Token, logger)

	optimalChunkSizeMB := int(getDefaultChunkSizeBytes(
		uint64(params.ArchiveSize), 8*1024*1024,
		100*1024*1024,
		uint64(getDefaultConcurrency())) / 1024 / 1024)

	logger.Debugf("Using multipart upload for file (%d bytes) with chunk size %d MB", params.ArchiveSize, optimalChunkSizeMB)
	logger.Debugf("Calculated chunk size: %d MB for file size: %d bytes (%d MB)", optimalChunkSizeMB, params.ArchiveSize, params.ArchiveSize/(1024*1024))

	err = u.uploadWithMultipart(ctx, params, validatedKey, client, logger, optimalChunkSizeMB)
	if err != nil {
		return fmt.Errorf("upload with multipart: %w", err)
	}

	return nil
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
		return fmt.Errorf("prepare multipart upload: %w", err)
	}

	logger.Debugf("Multipart Upload ID: %s", multipartResp.ID)
	logger.Debugf("Chunk count: %d, Chunk size: %d bytes", multipartResp.ChunkCount, multipartResp.ChunkSizeBytes)

	logger.Debugf("Upload chunks")
	etags, err := u.uploadChunks(ctx, params.ArchivePath, multipartResp, logger)
	if err != nil {
		logger.Warnf("Upload failed, aborting multipart upload %s", multipartResp.ID)
		if abortErr := client.abortMultipartUpload(multipartResp.ID); abortErr != nil {
			logger.Errorf("Failed to abort multipart upload: %v", abortErr)
		}
		return fmt.Errorf("upload chunks: %w", err)
	}

	logger.Debugf("Complete multipart upload")
	response, err := client.completeMultipartUpload(multipartResp.ID, etags)
	if err != nil {
		return fmt.Errorf("complete multipart upload: %w", err)
	}

	logger.Debugf("Multipart upload completed")
	logResponseMessage(response, logger)

	return nil
}

type chunkResult struct {
	index int
	etag  string
	err   error
}

type chunkReader struct {
	file          *os.File
	chunkSize     int64
	lastChunkSize int64
	numChunks     int
	mu            sync.Mutex
}

func (cr *chunkReader) readChunk(index int) ([]byte, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	size := cr.chunkSize
	if index == cr.numChunks-1 {
		size = cr.lastChunkSize
	}

	offset := int64(index) * cr.chunkSize
	_, err := cr.file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("seek to position %d for chunk %d: %w", offset, index+1, err)
	}

	chunk := make([]byte, size)
	n, err := io.ReadFull(cr.file, chunk)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("read chunk %d: %w", index+1, err)
	}

	if n == 0 {
		return nil, fmt.Errorf("unexpected end of file at chunk %d", index+1)
	}

	return chunk[:n], nil
}

func (cr *chunkReader) close() error {
	if cr.file != nil {
		return cr.file.Close()
	}
	return nil
}

type chunkUploadContext struct {
	stats               *chunkStatistics
	resultChan          chan chunkResult
	semaphore           chan struct{}
	numChunks           int
	maxRetryPerChunk    int
	chunkRetryThreshold time.Duration
	httpClient          *http.Client
}

func (c *chunkUploadContext) closeIdleConnections() {
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

func (u DefaultUploader) uploadChunks(ctx context.Context, archivePath string, response prepareMultipartUploadResponse, logger log.Logger) ([]string, error) {
	chunkReader, err := u.createChunkReader(archivePath, response)
	if err != nil {
		return nil, fmt.Errorf("create chunk reader: %w", err)
	}
	defer func() {
		if err := chunkReader.close(); err != nil {
			logger.Errorf("close chunk reader: %v", err)
		}
	}()

	etags, err := u.uploadAllChunks(ctx, chunkReader, response, logger)
	if err != nil {
		return nil, fmt.Errorf("upload all chunks: %w", err)
	}

	return etags, nil
}

func (u DefaultUploader) createChunkReader(archivePath string, response prepareMultipartUploadResponse) (*chunkReader, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive file: %w", err)
	}

	return &chunkReader{
		file:          file,
		chunkSize:     response.ChunkSizeBytes,
		lastChunkSize: response.LastChunkSizeBytes,
		numChunks:     len(response.URLs),
	}, nil
}

func (u DefaultUploader) uploadAllChunks(ctx context.Context, chunkReader *chunkReader, response prepareMultipartUploadResponse, logger log.Logger) ([]string, error) {
	numChunks := len(response.URLs)

	var stats chunkStatistics

	uploadCtx := &chunkUploadContext{
		stats:               &stats,
		resultChan:          make(chan chunkResult, numChunks),
		semaphore:           make(chan struct{}, getDefaultConcurrency()),
		numChunks:           numChunks,
		maxRetryPerChunk:    3,
		chunkRetryThreshold: 30 * time.Second,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     10 * time.Second,
				TLSHandshakeTimeout: 5 * time.Second,
				Proxy:               http.ProxyFromEnvironment,
			},
		},
	}
	defer uploadCtx.closeIdleConnections()

	for i, uploadURL := range response.URLs {
		go func(index int, url prepareMultipartUploadURL) {
			uploadCtx.semaphore <- struct{}{}
			defer func() { <-uploadCtx.semaphore }()

			chunkData, err := chunkReader.readChunk(index)
			if err != nil {
				uploadCtx.resultChan <- chunkResult{
					index: index,
					etag:  "",
					err:   fmt.Errorf("read chunk %d: %w", index+1, err),
				}
				return
			}

			etag, err := u.uploadChunkWithRetry(ctx, chunkData, url, index, uploadCtx, logger)
			uploadCtx.resultChan <- chunkResult{
				index: index,
				etag:  etag,
				err:   err,
			}
		}(i, uploadURL)
	}

	etags := make([]string, numChunks)
	completedChunks := 0
	for completedChunks < numChunks {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("upload cancelled while waiting for chunks: %w", ctx.Err())
		case result := <-uploadCtx.resultChan:
			completedChunks++
			if result.err != nil {
				return nil, fmt.Errorf("upload chunk %d after %d attempts: %w", result.index+1, uploadCtx.maxRetryPerChunk, result.err)
			}
			etags[result.index] = result.etag
		}
	}

	return etags, nil
}

func (u DefaultUploader) uploadChunkWithRetry(ctx context.Context, chunkData []byte, url prepareMultipartUploadURL, index int, uploadCtx *chunkUploadContext, logger log.Logger) (string, error) {
	var etag string
	var uploadErr error

	for attempt := 0; attempt < uploadCtx.maxRetryPerChunk; attempt++ {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("chunk %d upload cancelled: %w", index+1, ctx.Err())
		default:
		}

		logger.Debugf("Uploading chunk %d/%d (attempt %d/%d) [finished=%d] [avg=%v]",
			index+1, uploadCtx.numChunks, attempt+1, uploadCtx.maxRetryPerChunk,
			uploadCtx.stats.getFinishedCount(), uploadCtx.stats.average().Round(time.Second))

		start := time.Now()

		chunkCtx, cancelChunk := context.WithCancel(ctx)

		if attempt < uploadCtx.maxRetryPerChunk-1 {
			go func() {
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-chunkCtx.Done():
						return
					case <-ticker.C:
						if uploadCtx.stats.getFinishedCount() > 0 && time.Since(start)-uploadCtx.stats.average() > uploadCtx.chunkRetryThreshold {
							logger.Warnf("⚠️ Found hung chunk upload; canceling request after %s",
								time.Since(start).Round(time.Second))
							cancelChunk()
							return
						}
					}
				}
			}()
		}

		etag, uploadErr = u.uploadChunkWithContext(chunkCtx, url.Method, url.URL, url.Headers, chunkData, uploadCtx.httpClient, logger)
		cancelChunk()

		if uploadErr == nil {
			took := time.Since(start)
			uploadCtx.stats.update(took)
			logger.Infof("Chunk %d uploaded successfully in %v, ETag: %s",
				index+1, took.Round(time.Second), etag)
			break
		}

		logger.Warnf("Chunk %d attempt %d failed: %v", index+1, attempt+1, uploadErr)

		select {
		case <-ctx.Done():
			logger.Warnf("Chunk %d upload cancelled due to context cancellation", index+1)
			return "", fmt.Errorf("chunk %d upload cancelled: %w", index+1, ctx.Err())
		default:
			if chunkCtx.Err() == context.Canceled {
				logger.Warnf("Chunk %d attempt %d cancelled, retrying after %d seconds", index+1, attempt+1, (attempt+1)*2)
				time.Sleep(time.Duration((attempt+1)*2) * time.Second)
				continue
			}
		}
	}

	if uploadErr != nil {
		return etag, fmt.Errorf("upload chunk: %w", uploadErr)
	}
	return etag, nil
}

func (u DefaultUploader) uploadChunkWithContext(ctx context.Context, method, url string, headers map[string]string, chunk []byte, client *http.Client, logger log.Logger) (string, error) {

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(chunk))
	if err != nil {
		return "", fmt.Errorf("create chunk upload request: %w", err)
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
		return "", fmt.Errorf("upload chunk: %w", err)
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

	return etag, nil
}

func getDefaultChunkSizeBytes(totalSize, min, max, concurrency uint64) uint64 {

	cs := totalSize / concurrency

	if cs >= 100*1024*1024 {
		cs = cs / 2
	}

	if cs < min {
		cs = min
	}

	// Change ChunkSize if MaxChunkSize are set and ChunkSize > Max size
	if max > 0 && cs > max {
		cs = max
	}

	return cs
}

func getDefaultConcurrency() uint {
	c := uint(runtime.NumCPU() * 3)

	if c > 20 {
		c = 20
	}

	if c < 2 {
		c = 2
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
