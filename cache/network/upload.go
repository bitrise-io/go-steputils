package network

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-steputils/v2/cache/network/chunkuploader"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
)

// DefaultUploader ...
type DefaultUploader struct{}

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

	concurrency := chunkuploader.DefaultConcurrency()
	optimalChunkSizeMB := int(chunkuploader.OptimalChunkSizeBytes(params.ArchiveSize, concurrency) / 1024 / 1024)

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

func (u DefaultUploader) uploadChunks(ctx context.Context, archivePath string, response prepareMultipartUploadResponse, logger log.Logger) ([]string, error) {
	// Create file-based chunk provider
	provider, err := chunkuploader.NewFileChunkProvider(
		archivePath,
		response.ChunkSizeBytes,
		response.LastChunkSizeBytes,
		len(response.URLs),
	)
	if err != nil {
		return nil, fmt.Errorf("create chunk provider: %w", err)
	}
	defer func() {
		if err := provider.Close(); err != nil {
			logger.Errorf("close chunk provider: %v", err)
		}
	}()

	// Convert URLs to chunkuploader format
	urls := make([]chunkuploader.UploadURL, len(response.URLs))
	for i, u := range response.URLs {
		urls[i] = chunkuploader.UploadURL{
			Method:  u.Method,
			URL:     u.URL,
			Headers: u.Headers,
		}
	}

	// Create uploader with default config
	config := chunkuploader.DefaultConfig()

	uploader := chunkuploader.New(config)
	defer uploader.CloseIdleConnections()

	// Upload all chunks
	result, err := uploader.Upload(ctx, provider, urls)
	if err != nil {
		return nil, fmt.Errorf("upload chunks: %w", err)
	}

	return result.ETags, nil
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
