package network

import (
	"context"
	"errors"
	"fmt"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/melbahja/got"
	"net/http"
	"strings"
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

	retryableHTTPClient := prepareRetryableHTTPClient(logger)
	client := newAPIClient(retryableHTTPClient, params.APIBaseURL, params.Token, logger)

	logger.Debugf("Get download URL")
	restoreResponse, err := client.restore(params.CacheKeys)
	if err != nil {
		return "", fmt.Errorf("failed to get download URL: %w", err)
	}
	logger.Debugf("Download archive")
	downloadErr := downloadFile(ctx, retryableHTTPClient.StandardClient(), restoreResponse.URL, params.DownloadPath)
	if downloadErr == nil {
		return restoreResponse.MatchedKey, nil
	}
	return "", fmt.Errorf("failed to download archive")
}

func prepareRetryableHTTPClient(logger log.Logger) *retryablehttp.Client {
	retryableHTTPClient := retryhttp.NewClient(logger)
	retryableHTTPClient.CheckRetry = createCustomRetryFunction()
	return retryableHTTPClient
}

func createCustomRetryFunction() func(ctx context.Context, resp *http.Response, err error) (bool, error) {
	retriableErrors := []string{"Range request returned invalid Content-Length", "EOF", "connection reset"}

	return func(ctx context.Context, resp *http.Response, downloadErr error) (bool, error) {
		defaultCheckRetry := retryablehttp.DefaultRetryPolicy
		retry, err := defaultCheckRetry(ctx, resp, downloadErr)

		if !retry && err == nil && downloadErr != nil {
			for _, retriableError := range retriableErrors {
				if strings.Contains(downloadErr.Error(), retriableError) {
					return true, nil
				}
			}
		}
		return retry, err
	}
}

func downloadFile(ctx context.Context, client *http.Client, url string, dest string) error {
	downloader := got.New()
	downloader.Client = client
	return downloader.Do(got.NewDownload(ctx, url, dest))
}
