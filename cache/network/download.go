package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/melbahja/got"
)

// DownloadParams ...
type DownloadParams struct {
	APIBaseURL   string
	Token        string
	CacheKeys    []string
	DownloadPath string
}

var (
	// ErrCacheNotFound ...
	ErrCacheNotFound = errors.New("no cache archive found for the provided keys")

	retriableErrors = []string{"Range request returned invalid Content-Length", "unexpected EOF", "connection reset by peer"}
)

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
	retryableHTTPClient.CheckRetry = customRetryFunction
	client := newAPIClient(retryableHTTPClient, params.APIBaseURL, params.Token, logger)

	logger.Debugf("Get download URL")
	restoreResponse, err := client.restore(params.CacheKeys)
	if err != nil {
		return "", fmt.Errorf("failed to get download URL: %w", err)
	}

	logger.Debugf("Download archive")
	err = downloadFile(ctx, retryableHTTPClient.StandardClient(), restoreResponse.URL, params.DownloadPath)
	if err != nil {
		return "", fmt.Errorf("failed to download archive: %w", err)
	}

	return restoreResponse.MatchedKey, nil
}

// customRetryFunction - implements CheckRetry
//
// CheckRetry specifies a policy for handling retries. It is called
// following each request with the response and error values returned by
// the http.Client. If CheckRetry returns false, the Client stops retrying
// and returns the response to the caller. If CheckRetry returns an error,
// that error value is returned in lieu of the error from the request. The
// Client will close any response body when retrying, but if the retry is
// aborted it is up to the CheckRetry callback to properly close any
// response body before returning.
func customRetryFunction(ctx context.Context, resp *http.Response, downloadErr error) (bool, error) {
	// First call the DefaultRetryPolicy
	retry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, downloadErr)
	// If default policy says we should retry,
	// or if it returned with an error (it only returns err for context cancel - if we should NOT retry)
	// then return with what it returned with. It already said "do retry" or that we definitely shouldn't retry.
	if retry || err != nil {
		return retry, err
	}

	// In any other case, if the original downloadErr isn't nil
	// let's check it against our list of "retriable errors".
	if downloadErr != nil {
		for _, retriableError := range retriableErrors {
			if strings.Contains(downloadErr.Error(), retriableError) {
				return true, nil
			}
		}
	}

	return false, nil
}

func downloadFile(ctx context.Context, client *http.Client, url string, dest string) error {
	downloader := got.New()
	downloader.Client = client
	return downloader.Do(got.NewDownload(ctx, url, dest))
}
