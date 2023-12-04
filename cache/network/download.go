package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/melbahja/got"
)

// DownloadParams ...
type DownloadParams struct {
	APIBaseURL   string
	Token        string
	CacheKeys    []string
	DownloadPath string
}

// ErrCacheNotFound ...
var ErrCacheNotFound = errors.New("no cache archive found for the provided keys")

// Download archive from the cache API based on the provided keys in params.
// If there is no match for any of the keys, the error is ErrCacheNotFound.
func Download(ctx context.Context, params DownloadParams, logger log.Logger) (matchedKey string, err error) {
	if params.APIBaseURL == "" {
		return "", fmt.Errorf("API base URL is empty")
	}

	if params.Token == "" {
		return "", fmt.Errorf("API token is empty")
	}

	if len(params.CacheKeys) == 0 {
		return "", fmt.Errorf("cache key list is empty")
	}

	retryableHTTPClient := retryhttp.NewClient(logger)
	client := newAPIClient(retryableHTTPClient, params.APIBaseURL, params.Token, logger)

	logger.Debugf("Get download URL")
	restoreResponse, err := client.restore(params.CacheKeys)
	if err != nil {
		return "", fmt.Errorf("failed to get download URL: %w", err)
	}

	logger.Debugf("Download archive")

	const maxRetries = 3
	const retryDelay = 1 * time.Second
	retriableErrors := []string{"Range request returned invalid Content-Length", "EOF", "connection reset"}

	// Attempt to download, retrying on retriable errors
	for attempt := 0; attempt < maxRetries; attempt++ {
		downloadErr := downloadFile(ctx, retryableHTTPClient.StandardClient(), restoreResponse.URL, params.DownloadPath)

		if downloadErr == nil {
			return restoreResponse.MatchedKey, nil
		}

		isRetriable := false
		for _, retriableError := range retriableErrors {
			if strings.Contains(downloadErr.Error(), retriableError) {
				isRetriable = true
				break
			}
		}

		if !isRetriable {
			return "", fmt.Errorf("non-retriable error occurred: %w", downloadErr)
		}

		logger.Debugf("Retriable error occurred, attempt %d/%d: %v", attempt+1, maxRetries, downloadErr)

		if ctx.Err() != nil {
			return "", fmt.Errorf("context cancelled: %w", ctx.Err())
		}
		if attempt < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return "", fmt.Errorf("failed to download archive after %d attempts", maxRetries)
}

func downloadFile(ctx context.Context, client *http.Client, url string, dest string) error {
	downloader := got.New()
	downloader.Client = client

	return downloader.Do(got.NewDownload(ctx, url, dest))
}
