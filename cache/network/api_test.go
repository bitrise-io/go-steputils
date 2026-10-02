package network

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	endpoint string
	header   http.Header
	body     completeMultipartUploadRequest
}

func newRecordingAPIServer(t *testing.T) (*httptest.Server, func() []recordedRequest) {
	var (
		mu       sync.Mutex
		requests []recordedRequest
	)

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recordedRequest{endpoint: r.Method + " " + r.URL.Path, header: r.Header.Clone()}
		if r.Method == http.MethodPatch {
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&rec.body))
		}

		mu.Lock()
		requests = append(requests, rec)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/multipart-upload":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"upload-1"}`))
		case strings.HasSuffix(r.URL.Path, "/acknowledge"):
			_, _ = w.Write([]byte(`{"message":"ok"}`))
		case r.URL.Path == "/restore":
			_, _ = w.Write([]byte(`{"url":"https://example.com/archive","matched_cache_key":"key"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(svr.Close)

	return svr, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()

		return append([]recordedRequest(nil), requests...)
	}
}

func Test_apiClient_datacenterHeader(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		wantHeader string
		wantSet    bool
	}{
		{name: "sent when env var is set", envValue: "ORD1", wantHeader: "ORD1", wantSet: true},
		{name: "sent as-is when not a Bitrise DC", envValue: "US_EAST1", wantHeader: "US_EAST1", wantSet: true},
		{name: "trimmed", envValue: "  AMS1\n", wantHeader: "AMS1", wantSet: true},
		{name: "omitted when env var is empty", envValue: "", wantSet: false},
		{name: "omitted when env var is whitespace", envValue: "  ", wantSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(datacenterEnvKey, tt.envValue)

			svr, requests := newRecordingAPIServer(t)
			logger := log.NewLogger()
			client := newAPIClient(retryhttp.NewClient(logger), svr.URL, "token", logger)

			_, err := client.prepareMultipartUpload(prepareUploadRequest{CacheKey: "key"})
			require.NoError(t, err)
			_, err = client.completeMultipartUpload("upload-1", []string{"etag"})
			require.NoError(t, err)
			require.NoError(t, client.abortMultipartUpload("upload-2"))
			_, err = client.restore([]string{"key"})
			require.NoError(t, err)

			got := requests()
			require.Equal(t, []string{
				"POST /multipart-upload",
				"PATCH /multipart-upload/upload-1/acknowledge",
				"PATCH /multipart-upload/upload-2/acknowledge",
				"GET /restore",
			}, endpoints(got))
			assert.True(t, got[1].body.Successful, "complete")
			assert.False(t, got[2].body.Successful, "abort")

			for _, req := range got {
				assert.Equal(t, "Bearer token", req.header.Get("Authorization"), req.endpoint)
				values, ok := req.header[http.CanonicalHeaderKey(datacenterHeader)]
				assert.Equal(t, tt.wantSet, ok, req.endpoint)
				if tt.wantSet {
					assert.Equal(t, []string{tt.wantHeader}, values, req.endpoint)
				}
			}
		})
	}
}

func Test_apiClient_datacenterHeader_readPerRequest(t *testing.T) {
	svr, requests := newRecordingAPIServer(t)
	logger := log.NewLogger()
	client := newAPIClient(retryhttp.NewClient(logger), svr.URL, "token", logger)

	t.Setenv(datacenterEnvKey, "IAD1")
	_, err := client.restore([]string{"key"})
	require.NoError(t, err)

	t.Setenv(datacenterEnvKey, "ORD1")
	_, err = client.restore([]string{"key"})
	require.NoError(t, err)

	got := requests()
	require.Len(t, got, 2)
	assert.Equal(t, "IAD1", got[0].header.Get(datacenterHeader))
	assert.Equal(t, "ORD1", got[1].header.Get(datacenterHeader))
}

func Test_apiClient_datacenterHeader_keptOnRetry(t *testing.T) {
	t.Setenv(datacenterEnvKey, "AMS1")

	var (
		attempts atomic.Int32
		mu       sync.Mutex
		headers  []string
	)

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		headers = append(headers, r.Header.Get(datacenterHeader))
		mu.Unlock()

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://example.com/archive","matched_cache_key":"key"}`))
	}))
	defer svr.Close()

	logger := log.NewLogger()
	httpClient := retryhttp.NewClient(logger)
	httpClient.RetryWaitMin = 0
	httpClient.RetryWaitMax = 0
	client := newAPIClient(httpClient, svr.URL, "token", logger)

	_, err := client.restore([]string{"key"})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"AMS1", "AMS1"}, headers)
}

func endpoints(requests []recordedRequest) []string {
	out := make([]string, 0, len(requests))
	for _, r := range requests {
		out = append(out, r.endpoint)
	}

	return out
}
