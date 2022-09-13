package network

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
)

type UploadParams struct {
	APIBaseURL  string
	Token       string
	ArchivePath string
	ArchiveSize int64
	CacheKey    string
}

func Upload(params UploadParams, logger log.Logger) error {
	validatedKey, err := validateKey(params.CacheKey)
	if err != nil {
		return err
	}

	client := newApiClient(retryhttp.NewClient(logger), params.APIBaseURL, params.Token)

	logger.Debugf("Get upload URL")
	prepareUploadRequest := prepareUploadRequest{
		CacheKey:           validatedKey,
		ArchiveFileName:    filepath.Base(params.ArchivePath),
		ArchiveContentType: "application/zstd",
		ArchiveSizeInBytes: params.ArchiveSize,
	}
	resp, err := client.prepareUpload(prepareUploadRequest)
	if err != nil {
		return fmt.Errorf("failed to get upload URL: %w", err)
	}
	logger.Debugf("Upload ID: %s", resp.ID)

	logger.Println()
	logger.Debugf("Upload archive")
	err = client.uploadArchive(params.ArchivePath, resp.UploadMethod, resp.UploadURL, resp.UploadHeaders)
	if err != nil {
		return fmt.Errorf("failed to upload archive: %w", err)
	}

	logger.Println()
	logger.Debugf("Acknowledge upload")
	err = client.acknowledgeUpload(resp.ID)
	if err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}
	logger.Debugf("Upload acknowledged")

	return nil
}

func validateKey(key string) (string, error) {
	if strings.Contains(key, ",") {
		return "", fmt.Errorf("commas are not allowed in key")
	}

	if len(key) > maxKeyLength {
		return key[:maxKeyLength], nil
	}
	return key, nil
}
