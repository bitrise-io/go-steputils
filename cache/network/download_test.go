package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/mocks"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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

func Test_downloadFile(t *testing.T) {
	// Given
	mockLogger := new(mocks.Logger)
	mockLogger.On("Debugf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	retryableHTTPClient := retryhttp.NewClient(mockLogger)
	isCheckRetryCalled := false
	retryFunc := func(ctx context.Context, resp *http.Response, downloadErr error) (bool, error) {
		retry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, downloadErr)
		isCheckRetryCalled = true
		return retry, err
	}
	retryableHTTPClient.CheckRetry = retryFunc

	tmpPath := t.TempDir()
	tmpFile := filepath.Join(tmpPath, "testfile.bin")
	testDummyFileContent := strings.Repeat("a", 1024*1024*10) // 10MB

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
	// downloadURL := "https://github.com/bitrise-io/bitrise/releases/download/2.6.1/bitrise-Linux-x86_64"
	downloadURL := svr.URL

	// When
	err := downloadFile(context.Background(), retryableHTTPClient.StandardClient(), downloadURL, tmpFile)

	// Then
	require.True(t, isCheckRetryCalled)
	require.NoError(t, err)
	mockLogger.AssertExpectations(t)
}
