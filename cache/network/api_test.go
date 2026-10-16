package network

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

			_, err := client.prepareMultipartUpload(prepareUploadRequest{CacheKey: "key"}, false)
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

func Test_prepareMultipartUpload(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		want        prepareMultipartUploadResponse
		wantSkipped *UploadSkippedError
		wantErr     bool
	}{
		{
			name:   "201 returns upload URLs",
			status: http.StatusCreated,
			body:   `{"id":"upload-id","chunk_size_bytes":10,"chunk_count":1,"last_chunk_size_bytes":10,"urls":[{"method":"PUT","url":"https://storage/part1"}]}`,
			want: prepareMultipartUploadResponse{
				ID: "upload-id", ChunkSizeBytes: 10, ChunkCount: 1, LastChunkSizeBytes: 10,
				URLs: []prepareMultipartUploadURL{{Method: "PUT", URL: "https://storage/part1"}},
			},
		},
		{
			name:        "200 with skipped:true returns UploadSkippedError",
			status:      http.StatusOK,
			body:        `{"skipped":true,"reason":"identical_key_upload_in_progress","existing_id":"existing-id"}`,
			wantSkipped: &UploadSkippedError{Reason: "identical_key_upload_in_progress", ExistingID: "existing-id"},
		},
		{
			name:    "200 without skipped:true is an error",
			status:  http.StatusOK,
			body:    `{"id":"upload-id","urls":[]}`,
			wantErr: true,
		},
		{
			name:    "200 with invalid body is an error",
			status:  http.StatusOK,
			body:    `not json`,
			wantErr: true,
		},
		{
			name:    "400 is an error",
			status:  http.StatusBadRequest,
			body:    `{"message":"bad request"}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotSkipHeader string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/multipart-upload", r.URL.Path)
				gotSkipHeader = r.Header.Get(uploadSkipHeader)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			logger := log.NewLogger()
			client := newAPIClient(newUploadHTTPClient(logger), server.URL, "token", logger)

			got, err := client.prepareMultipartUpload(prepareUploadRequest{CacheKey: "key", ArchiveSizeInBytes: 10}, true)

			assert.Equal(t, "1", gotSkipHeader)
			switch {
			case tt.wantSkipped != nil:
				require.ErrorIs(t, err, ErrUploadSkipped)
				var skipped UploadSkippedError
				require.True(t, errors.As(err, &skipped))
				assert.Equal(t, *tt.wantSkipped, skipped)
			case tt.wantErr:
				require.Error(t, err)
				assert.NotErrorIs(t, err, ErrUploadSkipped)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_Upload_serverSkip(t *testing.T) {
	var acknowledged bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			acknowledged = true
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"skipped":true,"reason":"identical_key_recently_uploaded","existing_id":"existing-id"}`))
	}))
	defer server.Close()

	err := DefaultUploader{}.Upload(context.Background(), UploadParams{
		APIBaseURL:      server.URL,
		Token:           "token",
		ArchivePath:     "/does/not/exist.tzst", // never opened on skip
		ArchiveSize:     10,
		CacheKey:        "key",
		AllowServerSkip: true,
	}, log.NewLogger())

	require.ErrorIs(t, err, ErrUploadSkipped)
	assert.False(t, acknowledged, "a skipped upload must not be acknowledged")
}

func Test_prepareMultipartUpload_skipHeader(t *testing.T) {
	newServer := func(statuses ...int) (*httptest.Server, *[]string) {
		var headers []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers = append(headers, r.Header.Get(uploadSkipHeader))
			w.WriteHeader(statuses[len(headers)-1])
			_, _ = w.Write([]byte(`{"id":"upload-id"}`))
		}))

		return server, &headers
	}
	newClient := func(url string) apiClient {
		logger := log.NewLogger()
		httpClient := newUploadHTTPClient(logger)
		httpClient.RetryWaitMin, httpClient.RetryWaitMax = time.Millisecond, time.Millisecond

		return newAPIClient(httpClient, url, "token", logger)
	}

	t.Run("not sent unless the caller opts in", func(t *testing.T) {
		server, headers := newServer(http.StatusCreated)
		defer server.Close()

		_, err := newClient(server.URL).prepareMultipartUpload(prepareUploadRequest{CacheKey: "key"}, false)

		require.NoError(t, err)
		assert.Equal(t, []string{""}, *headers)
	})

	t.Run("sent on the first attempt only", func(t *testing.T) {
		server, headers := newServer(http.StatusBadGateway, http.StatusCreated)
		defer server.Close()

		_, err := newClient(server.URL).prepareMultipartUpload(prepareUploadRequest{CacheKey: "key"}, true)

		require.NoError(t, err)
		assert.Equal(t, []string{"1", ""}, *headers, "a retry must not ask for a skip it could get against its own upload")
	})

	t.Run("retries keep the datacenter header", func(t *testing.T) {
		t.Setenv(datacenterEnvKey, "ORD1")

		var dcs []string
		attempt := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dcs = append(dcs, r.Header.Get(datacenterHeader))
			attempt++
			if attempt == 1 {
				w.WriteHeader(http.StatusBadGateway)

				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"upload-id"}`))
		}))
		defer server.Close()

		_, err := newClient(server.URL).prepareMultipartUpload(prepareUploadRequest{CacheKey: "key"}, true)

		require.NoError(t, err)
		assert.Equal(t, []string{"ORD1", "ORD1"}, dcs)
	})
}
