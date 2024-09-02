package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/bitrise-io/got"
	"github.com/hashicorp/go-retryablehttp"
)

// DefaultDownloader ...
type DefaultDownloader struct{}

// DownloadParams ...
type DownloadParams struct {
	APIBaseURL     string
	Token          string
	CacheKeys      []string
	DownloadPath   string
	NumFullRetries int
	MaxConcurrency uint
}

// ErrCacheNotFound ...
var ErrCacheNotFound = errors.New("no cache archive found for the provided keys")

// Download archive from the cache API based on the provided keys in params.
// If there is no match for any of the keys, the error is ErrCacheNotFound.
func (d DefaultDownloader) Download(ctx context.Context, params DownloadParams, logger log.Logger) (string, error) {
	retryableHTTPClient := retryhttp.NewClient(logger)

	env := os.Getenv("R2_MAX_IDLE_CONNS_PER_HOST")
	maxIdleConnsPerHost, err := strconv.Atoi(env)
	if err == nil {
		retryableHTTPClient.HTTPClient.Transport.(*http.Transport).MaxIdleConnsPerHost = maxIdleConnsPerHost
	}

	env = os.Getenv("R2_MAX_IDLE_CONNS")
	maxIdleConns, err := strconv.Atoi(env)
	if err == nil {
		retryableHTTPClient.HTTPClient.Transport.(*http.Transport).MaxIdleConns = maxIdleConns
	}

	env = os.Getenv("R2_FORCE_ATTEMPT_HTTP2")
	forceAttemptHTTP2 := env == "true" || env == "1"
	retryableHTTPClient.HTTPClient.Transport.(*http.Transport).ForceAttemptHTTP2 = forceAttemptHTTP2

	env = os.Getenv("R2_DUALSTACK")
	dualStack := env == "true" || env == "1"
	retryableHTTPClient.HTTPClient.Transport.(*http.Transport).DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: dualStack,
	}).DialContext

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
		downloadErr := downloadFile(ctx, httpClient.StandardClient(), restoreResponse.URL, params.DownloadPath, params.MaxConcurrency, logger)
		if downloadErr != nil {
			logger.Debugf("Failed to download archive: %s", downloadErr)
			return fmt.Errorf("failed to download archive: %w", downloadErr), false
		}

		matchedKey = restoreResponse.MatchedKey
		return nil, false
	})

	return matchedKey, err
}

func downloadFile(ctx context.Context, client *http.Client, url string, dest string, maxConcurrency uint, logger log.Logger) error {
	downloader := got.New()
	downloader.Client = client

	gDownload := got.NewDownload(ctx, url, dest)
	// Client has to be set on "Download" as well,
	// as depending on how downloader is called
	// either the Client from the downloader or from the Download will be used.
	gDownload.Client = client
	gDownload.Concurrency = maxConcurrency

	gDownload.Logger = logger

	err := downloader.Do(gDownload)

	return err
}
