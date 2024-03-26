package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/melbahja/got"
)

// DownloadParams ...
type DownloadParams struct {
	APIBaseURL     string
	Token          string
	CacheKeys      []string
	DownloadPath   string
	NumFullRetries int
	BuildCacheURL  string
	AppSlug        string
}

// ErrCacheNotFound ...
var ErrCacheNotFound = errors.New("no cache archive found for the provided keys")

// Download archive from the cache API based on the provided keys in params.
// If there is no match for any of the keys, the error is ErrCacheNotFound.
func Download(ctx context.Context, params DownloadParams, logger log.Logger) (string, error) {
	retryableHTTPClient := retryhttp.NewClient(logger)
	return downloadWithClient(ctx, retryableHTTPClient, params, logger)
}

func downloadWithClient(ctx context.Context, httpClient *retryablehttp.Client, params DownloadParams, logger log.Logger) (string, error) {
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
	err := retry.Times(uint(params.NumFullRetries)).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		if attempt != 0 {
			logger.Debugf("Retrying archive download... (attempt %d)", attempt+1)
		}

		client := newAPIClient(httpClient, params.APIBaseURL, params.Token, logger)

		logger.Debugf("Fetching download URL...")
		restoreResponse, err := client.restore(params.CacheKeys)
		if err != nil {
			if errors.Is(err, ErrCacheNotFound) {
				return err, true // Do not retry if cache key not found
			}

			logger.Debugf("Failed to get download URL: %s", err)
			return fmt.Errorf("failed to get download URL: %w", err), false
		}

		logger.Debugf("Downloading archive...")
		url, err := buildCacheKeyURL(buildCacheKeyURLParams{
			serviceURL: params.BuildCacheURL,
			appSlug:    params.AppSlug,
			id:         restoreResponse.MatchedKey,
		})
		if err != nil {
			return fmt.Errorf("generate build cache url: %w", err), false
		}
		downloadErr := downloadFile(ctx, httpClient.StandardClient(), url, params.DownloadPath, params.Token)
		if downloadErr != nil {
			notFoundText := "Response status code is not ok: 404"
			if strings.Contains(downloadErr.Error(), notFoundText) {
				return ErrCacheNotFound, true
			}
			logger.Debugf("Failed to download archive: %s", downloadErr)
			return fmt.Errorf("failed to download archive: %w", downloadErr), false
		}

		matchedKey = restoreResponse.MatchedKey
		return nil, false
	})

	return matchedKey, err
}

func downloadFile(ctx context.Context, client *http.Client, url, dest, token string) error {
	downloader := got.New()
	downloader.Client = client

	gDownload := got.NewDownload(ctx, url, dest)
	// Client has to be set on "Download" as well,
	// as depending on how downloader is called
	// either the Client from the downloader or from the Download will be used.
	gDownload.Client = client
	gDownload.Header = append(gDownload.Header, got.GotHeader{
		Key:   "Authorization",
		Value: fmt.Sprintf("Bearer %s", token),
	})

	err := downloader.Do(gDownload)

	return err
}
