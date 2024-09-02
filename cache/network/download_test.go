package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/mocks"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/docker/go-units"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func customRetryFunction(ctx context.Context, resp *http.Response, err error) (bool, error) {
	return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
}

func TestCreateCustomRetryFunction(t *testing.T) {
	cases := []struct {
		name     string
		response *http.Response
		ctx      context.Context

		error    error
		expected bool
	}{
		{
			name:     "Retry for retriable error",
			response: &http.Response{},
			error:    errors.New("EOF"),
			expected: true,
		},
		{
			name:     "Retry for any error",
			response: &http.Response{},
			error:    errors.New("non-pattern-matching-error"),
			expected: true,
		},
		{
			name:     "Retry for retriable error",
			response: &http.Response{},
			error:    errors.New("Range request returned invalid Content-Length"),
			expected: true,
		},
		{
			name:     "No retry for HTTP 404 status code",
			response: &http.Response{StatusCode: 404},
			error:    nil,
			expected: false,
		},
		{
			name:     "Retry, even though the status is non-retriable in default policy",
			response: &http.Response{StatusCode: 404},
			error:    errors.New("Range request returned invalid Content-Length"),
			expected: true,
		},
		{
			name:     "Retry, even though the status is 404 and error pattern isnt matching",
			response: &http.Response{StatusCode: 404},
			error:    errors.New("non-pattern-matching-error"),
			expected: true,
		},
		{
			name:     "Retry for HTTP 429 status code",
			response: &http.Response{StatusCode: 429},
			error:    nil,
			expected: true,
		},
		{
			name:     "Retry for HTTP 500 status code",
			response: &http.Response{StatusCode: 500},
			error:    nil,
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			retry, _ := customRetryFunction(context.Background(), tc.response, tc.error)
			assert.Equal(t, tc.expected, retry)
		})
	}
}

func Test_downloadFile_multipart_retrycheck(t *testing.T) {
	// Given
	mockLogger := new(mocks.Logger)
	mockLogger.On("Debugf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	retryableHTTPClient := retryhttp.NewClient(mockLogger)

	var isCheckRetryCalled atomic.Bool
	retryFunc := func(ctx context.Context, resp *http.Response, downloadErr error) (bool, error) {
		retry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, downloadErr)
		isCheckRetryCalled.Store(true)
		return retry, err
	}
	retryableHTTPClient.CheckRetry = retryFunc

	tmpPath := t.TempDir()
	tmpFile := filepath.Join(tmpPath, "testfile.bin")
	testDummyFileContent := strings.Repeat("a", 10*units.MB) // 10MB

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Server called. Method=%s; Header=%#v", r.Method, r.Header)
		rangeHeader := r.Header.Get("Range")
		if len(rangeHeader) < 1 {
			t.Fatal("No Range header found")
		}

		if !strings.HasPrefix(rangeHeader, "bytes=") {
			t.Fatalf("invalid range header: should start with 'bytes=' ; actual range header value was=%s", rangeHeader)
		}
		rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
		rangeHeaderFromTo := strings.Split(rangeHeader, "-")
		if len(rangeHeaderFromTo) != 2 {
			t.Fatalf("invalid range header: invalid from-to value. Range header value was=%s", rangeHeader)
		}
		rangeHeaderFrom, err := strconv.ParseUint(rangeHeaderFromTo[0], 10, 64)
		require.NoError(t, err)
		rangeHeaderTo, err := strconv.ParseUint(rangeHeaderFromTo[1], 10, 64)
		require.NoError(t, err)

		if rangeHeaderFrom == 0 && rangeHeaderTo == 0 {
			// range request - requesting content size - return the size info
			w.Header().Add("content-range", fmt.Sprintf("bytes 0-0/%d", len(testDummyFileContent)))
			_, err := fmt.Fprint(w, " ")
			require.NoError(t, err)
		} else {
			// actual content chunk request - return chunk content
			chunkContent := testDummyFileContent[rangeHeaderFrom : rangeHeaderTo+1]
			// We also have to set the Content-Length header manually due to the size of the response.
			// From the documentation of http.ResponseWriter:
			// > ... if the total size of all written
			// > data is under a few KB and there are no Flush calls, the
			// > Content-Length header is added automatically.
			w.Header().Add("Content-Length", fmt.Sprintf("%d", len(chunkContent)))
			_, err := fmt.Fprint(w, chunkContent)
			require.NoError(t, err)
		}
	}))
	defer svr.Close()
	downloadURL := svr.URL

	// When
	err := downloadFile(context.Background(), retryableHTTPClient, downloadURL, tmpFile, 5, log.NewLogger())

	// Then
	require.True(t, isCheckRetryCalled.Load())
	require.NoError(t, err)
	mockLogger.AssertExpectations(t)
}

func Test_downloadWithClient_WhenNoChunksError_ThenWillDoFullRetry(t *testing.T) {
	// Given
	logger := log.NewLogger()
	logger.EnableDebugLog(true)

	retryableHTTPClient := retryhttp.NewClient(logger)
	retryFunc := func(ctx context.Context, resp *http.Response, downloadErr error) (bool, error) {
		return false, downloadErr // Disable retries
	}
	retryableHTTPClient.CheckRetry = retryFunc

	var numErrorsLeft atomic.Int64
	numErrorsLeft.Store(2)

	tmpPath := t.TempDir()
	tmpFile := filepath.Join(tmpPath, "testfile.bin")
	testDummyFileContent := strings.Repeat("a", 10*units.MB) // 10MB
	cacheKey := "test-cache-key"

	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("[fileserver] Server called. Method=%s; Header=%#v", r.Method, r.Header)
		if numErrorsLeft.Load() > 0 {
			numErrorsLeft.Add(-1)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		// We also have to set the Content-Length header manually due to the size of the response.
		// From the documentation of http.ResponseWriter:
		// > ... if the total size of all written
		// > data is under a few KB and there are no Flush calls, the
		// > Content-Length header is added automatically.
		w.Header().Add("Content-Length", fmt.Sprintf("%d", len(testDummyFileContent)))
		_, err := fmt.Fprint(w, testDummyFileContent)
		require.NoError(t, err)
	}))
	defer fileServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("[apiserver] Server called. Path=%s; Method=%s; Query=%s; Header=%#v", r.URL.Path, r.Method, r.URL.Query(), r.Header)

		resp := restoreResponse{
			URL:        fileServer.URL,
			MatchedKey: cacheKey,
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer apiServer.Close()

	downloadParams := DownloadParams{
		APIBaseURL:     apiServer.URL,
		Token:          "netok",
		CacheKeys:      []string{cacheKey},
		DownloadPath:   tmpFile,
		NumFullRetries: 3,
	}

	// When
	gotMatchedKeys, err := downloadWithClient(context.Background(), retryableHTTPClient, downloadParams, logger)
	// Then
	require.NoError(t, err)
	require.Equal(t, "test-cache-key", gotMatchedKeys)

	downloadedContents, err := os.ReadFile(tmpFile) // Read back downloaded file
	require.NoError(t, err)
	require.Equal(t, testDummyFileContent, string(downloadedContents), "Contents should match")
	require.Equal(t, numErrorsLeft.Load(), int64(0), "Numbers of retries is number errors + the final successful attempt")
}

func Test_downloadWithClient_WhenCacheKeyNotFound_ThenWillNotRetry(t *testing.T) {
	// Given
	logger := log.NewLogger()
	logger.EnableDebugLog(true)

	retryableHTTPClient := retryhttp.NewClient(logger)
	retryFunc := func(ctx context.Context, resp *http.Response, downloadErr error) (bool, error) {
		retry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, downloadErr)
		require.False(t, retry, "Should not retry")
		return retry, err
	}
	retryableHTTPClient.CheckRetry = retryFunc

	var apiServerCalled atomic.Uint64
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("[apiserver] Server called. Path=%s; Method=%s; Query=%s; Header=%#v", r.URL.Path, r.Method, r.URL.Query(), r.Header)
		apiServerCalled.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiServer.Close()

	downloadParams := DownloadParams{
		APIBaseURL:     apiServer.URL,
		Token:          "netok",
		CacheKeys:      []string{"test-cache--key"},
		DownloadPath:   "",
		NumFullRetries: 3,
	}

	// When
	gotMatchedKeys, err := downloadWithClient(context.Background(), retryableHTTPClient, downloadParams, logger)
	// Then
	require.ErrorContains(t, err, "no cache archive found for the provided keys")
	require.Equal(t, "", gotMatchedKeys)

	require.Equal(t, uint64(1), apiServerCalled.Load(), "no retries were done")
}
