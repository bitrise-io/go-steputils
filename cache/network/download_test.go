package network

import (
	"context"
	"errors"
	"net/http"
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

	downloadURL := "https://github.com/bitrise-io/bitrise/releases/download/2.6.1/bitrise-Linux-x86_64"
	retryableHTTPClient := retryhttp.NewClient(mockLogger)

	isCheckRetryCalled := false
	retryFunc := func(ctx context.Context, resp *http.Response, downloadErr error) (bool, error) {
		retry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, downloadErr)
		isCheckRetryCalled = true
		return retry, err
	}
	retryableHTTPClient.CheckRetry = retryFunc

	// When
	err := downloadFile(context.Background(), retryableHTTPClient.StandardClient(), downloadURL, "/tmp/t.file")

	// Then
	require.NoError(t, err)
	require.True(t, isCheckRetryCalled)
	mockLogger.AssertExpectations(t)
}
