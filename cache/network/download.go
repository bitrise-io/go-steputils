package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bitrise-io/go-utils/retry"
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

	downloadErr := downloadFile(ctx, retryableHTTPClient.StandardClient(), restoreResponse.URL, params.DownloadPath, logger)
	if downloadErr != nil {
		return "", fmt.Errorf("failed to download archive: %w", downloadErr)
	}

	return restoreResponse.MatchedKey, nil
}

func downloadFile(ctx context.Context, client *http.Client, url string, dest string, logger log.Logger) error {
	return retry.Times(5).Wait(5 * time.Second).Try(func(attempt uint) error {
		downloader := got.New()
		downloader.Client = client

		gDownload := got.NewDownload(ctx, url, dest)
		// Client has to be set on "Download" as well,
		// as depending on how downloader is called
		// either the Client from the downloader or from the Download will be used.
		gDownload.Client = client

		err := downloader.Do(gDownload)
		if err != nil {
			logger.Debugf("Archive download failed: %v (attempt %d)", err, attempt+1)
		}
		return err
	})
}
