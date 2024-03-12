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
func Download(ctx context.Context, params DownloadParams, logger log.Logger) (string, error) {
	if params.APIBaseURL == "" {
		return "", fmt.Errorf("API base URL is empty")
	}

	if params.Token == "" {
		return "", fmt.Errorf("API token is empty")
	}

	if len(params.CacheKeys) == 0 {
		return "", fmt.Errorf("cache key list is empty")
	}

	matchedKey := ""
	err := retry.Times(5).Wait(5 * time.Second).Try(func(attempt uint) error {
		if attempt != 0 {
			logger.Debugf("Archive download attempt %d", attempt+1)
		}

		retryableHTTPClient := retryhttp.NewClient(logger)
		client := newAPIClient(retryableHTTPClient, params.APIBaseURL, params.Token, logger)

		logger.Debugf("Fetching download URL...")
		restoreResponse, err := client.restore(params.CacheKeys)
		if err != nil {
			logger.Debugf("Failed to get download URL: %w", err)
			return fmt.Errorf("failed to get download URL: %w", err)
		}

		logger.Debugf("Downloading archive...")
		downloadErr := downloadFile(ctx, retryableHTTPClient.StandardClient(), restoreResponse.URL, params.DownloadPath, logger)
		if downloadErr != nil {
			logger.Debugf("Failed to download archive: %w", downloadErr)
			return fmt.Errorf("failed to download archive: %w", downloadErr)
		}

		matchedKey = restoreResponse.MatchedKey
		return nil
	})

	return matchedKey, err
}

func downloadFile(ctx context.Context, client *http.Client, url string, dest string, logger log.Logger) error {
	downloader := got.New()
	downloader.Client = client

	gDownload := got.NewDownload(ctx, url, dest)
	// Client has to be set on "Download" as well,
	// as depending on how downloader is called
	// either the Client from the downloader or from the Download will be used.
	gDownload.Client = client

	err := downloader.Do(gDownload)

	return err
}
