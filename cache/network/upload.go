package network

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
)

// UploadParams ...
type UploadParams struct {
	APIBaseURL    string
	Token         string
	ArchivePath   string
	ArchiveSize   int64
	CacheKey      string
	BuildCacheURL string
	AppSlug       string
	Sha256Sum     string
}

// Upload a cache archive and associate it with the provided cache key
func Upload(params UploadParams, logger log.Logger) error {
	validatedKey, err := validateKey(params.CacheKey, logger)
	if err != nil {
		return err
	}

	client := newAPIClient(retryhttp.NewClient(logger), params.APIBaseURL, params.Token, logger)

	logger.Debugf("Get upload URL")
	prepareUploadRequest := prepareUploadRequest{
		CacheKey:           validatedKey,
		ArchiveFileName:    filepath.Base(params.ArchivePath),
		ArchiveContentType: "text/plain",
		ArchiveSizeInBytes: 1,
	}
	resp, err := client.prepareUpload(prepareUploadRequest)
	if err != nil {
		return fmt.Errorf("failed to get upload URL: %w", err)
	}
	logger.Debugf("Upload ID: %s", resp.ID)

	logger.Debugf("")
	logger.Debugf("Upload archive")
	uploadFlagFile, err := os.CreateTemp("", "uploadflag")
	if err != nil {
		return fmt.Errorf("create flag file: %w", err)
	}
	defer os.Remove(uploadFlagFile.Name())
	if _, err := uploadFlagFile.Write([]byte{42}); err != nil {
		return fmt.Errorf("write flag file: %w", err)
	}
	err = client.uploadArchive(uploadFlagFile.Name(), resp.UploadMethod, resp.UploadURL, resp.UploadHeaders)
	if err != nil {
		return fmt.Errorf("upload flag file: %w", err)
	}
	url, err := buildCacheKeyURL(buildCacheKeyURLParams{
		serviceURL: params.BuildCacheURL,
		appSlug:    params.AppSlug,
		id:         validatedKey,
	})
	if err != nil {
		return fmt.Errorf("generate build cache url: %w", err)
	}
	buildCacheHeaders := map[string]string{
		"Authorization":                    fmt.Sprintf("Bearer %s", params.Token),
		"x-flare-blob-validation-sha256":   params.Sha256Sum,
		"x-flare-blob-validation-level":    "error",
		"x-flare-no-skip-duplicate-writes": "true",
	}
	err = client.uploadArchive(params.ArchivePath, http.MethodPut, url, buildCacheHeaders)
	if err != nil {
		return fmt.Errorf("failed to upload archive: %w", err)
	}

	logger.Debugf("")
	logger.Debugf("Acknowledge upload")
	response, err := client.acknowledgeUpload(resp.ID)
	if err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	logger.Debugf("Upload acknowledged")
	logResponseMessage(response, logger)

	return nil
}

type buildCacheKeyURLParams struct {
	serviceURL string
	appSlug    string
	id         string
}

func buildCacheKeyURL(p buildCacheKeyURLParams) (string, error) {
	h := sha256.New()
	if _, err := h.Write([]byte(p.id)); err != nil {
		return "", fmt.Errorf("write sha256: %w", err)
	}
	buildCacheKey := fmt.Sprintf("%x", h.Sum(nil))
	return fmt.Sprintf(
		"%s/cache/%s/%s",
		p.serviceURL, p.appSlug, buildCacheKey,
	), nil
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
