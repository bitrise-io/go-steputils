package redact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/require"
)

const presignedURL = "https://bucket.s3.amazonaws.com/cache/archive.tzst?X-Amz-Expires=3600&X-Amz-Signature=5ecre7"

func TestError_urlError(t *testing.T) {
	cause := context.DeadlineExceeded
	urlErr := &url.Error{Op: "Get", URL: presignedURL, Err: cause}

	got := Error(urlErr)

	require.EqualError(t, got, `Get "<redacted URL>": context deadline exceeded`)
	require.ErrorIs(t, got, cause)
	var asURLErr *url.Error
	require.ErrorAs(t, got, &asURLErr)
	require.Equal(t, "<redacted URL>", asURLErr.URL)
	require.True(t, asURLErr.Timeout())
	require.Equal(t, presignedURL, urlErr.URL, "the original is left alone")
}

// What http.NewRequest returns for a URL it can't parse. Nothing here depends
// on the URL being well-formed.
func TestError_unparsableURL(t *testing.T) {
	parseErr := &url.Error{Op: "parse", URL: "https://host.example/a b/%zz?sig=5ecre7", Err: url.EscapeError("%zz")}

	got := Error(parseErr)

	require.EqualError(t, got, `parse "<redacted URL>": invalid URL escape "%zz"`)
}

// retryablehttp's default error handler formats the URL into its own message,
// and http.Client wraps that in a *url.Error.
func TestError_urlInMessageOfCause(t *testing.T) {
	cause := errors.New("connection refused")
	givingUp := fmt.Errorf("GET %s giving up after 3 attempt(s): %w", presignedURL, cause)

	got := Error(&url.Error{Op: "Get", URL: presignedURL, Err: givingUp})

	require.EqualError(t, got, `Get "<redacted URL>": GET <redacted URL> giving up after 3 attempt(s): connection refused`)
	require.ErrorIs(t, got, cause)
}

func TestError_withoutURLIsUntouched(t *testing.T) {
	require.NoError(t, Error(nil))

	err := fmt.Errorf("read body: %w", io.ErrUnexpectedEOF)
	require.Same(t, err, Error(err))
}

func TestLogger(t *testing.T) {
	var out bytes.Buffer
	logger := Logger(log.NewLogger(log.WithOutput(&out), log.WithDebugLog(true)))

	urlErr := &url.Error{Op: "Get", URL: presignedURL, Err: io.EOF}
	parsed, err := url.Parse(presignedURL)
	require.NoError(t, err)

	logger.Debugf("[DEBUG] %s %s", "GET", presignedURL)
	logger.Debugf("[DEBUG] GET %s: retrying in %s (%d left)", parsed, "1s", 2)
	logger.Debugf("[ERR] GET %s request failed: %v", presignedURL, urlErr)
	logger.Infof("%s", presignedURL)
	logger.Warnf("%s", presignedURL)
	logger.Errorf("%s", presignedURL)
	logger.Printf("%s", presignedURL)
	logger.Donef("%s", presignedURL)
	logger.TInfof("%s", presignedURL)
	logger.TWarnf("%s", presignedURL)
	logger.TErrorf("%s", presignedURL)
	logger.TPrintf("%s", presignedURL)
	logger.TDonef("%s", presignedURL)
	logger.TDebugf("%s", presignedURL)
	logger.Debugf("100%% of %d chunks", 4)

	logged := out.String()
	require.NotContains(t, logged, "amazonaws")
	require.NotContains(t, logged, "5ecre7")
	lines := strings.Split(strings.TrimSpace(logged), "\n")
	require.Len(t, lines, 15)
	for _, line := range lines[:14] {
		require.Contains(t, line, "<redacted URL>")
	}
	require.Contains(t, logged, "GET <redacted URL>: retrying in 1s (2 left)")
	require.Contains(t, logged, `request failed: Get "<redacted URL>": EOF`)
	require.Contains(t, logged, "100% of 4 chunks")
}
