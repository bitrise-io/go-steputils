package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bitrise-io/go-steputils/v2/cache/kv"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/hashicorp/go-retryablehttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DownloadParams ...
type DownloadParams struct {
	APIBaseURL     string
	Token          string
	CacheKeys      []string
	DownloadPath   string
	NumFullRetries int
	BuildCacheHost string
	InsecureGRPC   bool
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
		buildCacheKey, err := buildCacheKey(restoreResponse.MatchedKey)
		if err != nil {
			return fmt.Errorf("generate build cache key: %w", err), false
		}

		file, err := os.Create(params.DownloadPath)
		if err != nil {
			return fmt.Errorf("create %q: %w", params.DownloadPath, err), false
		}
		defer file.Close()

		kvClient, err := kv.NewClient(ctx, kv.NewClientParams{
			UseInsecure: params.InsecureGRPC,
			Host:        params.BuildCacheHost,
			DialTimeout: 5 * time.Second,
			ClientName:  "kv",
			Token:       params.Token,
		})
		if err != nil {
			return fmt.Errorf("new kv client: %w", err), false
		}
		kvReader, err := kvClient.Get(ctx, buildCacheKey)
		if err != nil {
			return fmt.Errorf("create kv get client: %w", err), false
		}
		defer kvReader.Close()

		if _, err := io.Copy(file, kvReader); err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.NotFound {
				return ErrCacheNotFound, true
			}
			logger.Debugf("Failed to download archive: %s", err)
			return fmt.Errorf("failed to download archive: %w", err), false
		}

		matchedKey = restoreResponse.MatchedKey
		return nil, false
	})

	return matchedKey, err
}
