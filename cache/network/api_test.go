package network

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_apiClient_datacenterHeader(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		wantHeader string
		wantSet    bool
	}{
		{name: "sent when env var is set", envValue: "ORD1", wantHeader: "ORD1", wantSet: true},
		{name: "trimmed", envValue: "  AMS1\n", wantHeader: "AMS1", wantSet: true},
		{name: "omitted when env var is empty", envValue: "", wantSet: false},
		{name: "omitted when env var is whitespace", envValue: "  ", wantSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(datacenterEnvKey, tt.envValue)

			var mu sync.Mutex
			seen := map[string]http.Header{}
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				seen[r.Method+" "+r.URL.Path] = r.Header.Clone()
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
			defer svr.Close()

			logger := log.NewLogger()
			client := newAPIClient(retryhttp.NewClient(logger), svr.URL, "token", logger)

			_, err := client.prepareMultipartUpload(prepareUploadRequest{CacheKey: "key"})
			require.NoError(t, err)
			_, err = client.completeMultipartUpload("upload-1", []string{"etag"})
			require.NoError(t, err)
			_, err = client.restore([]string{"key"})
			require.NoError(t, err)

			require.Len(t, seen, 3)
			for endpoint, header := range seen {
				assert.Equal(t, "Bearer token", header.Get("Authorization"), endpoint)
				values, ok := header[http.CanonicalHeaderKey(datacenterHeader)]
				assert.Equal(t, tt.wantSet, ok, endpoint)
				if tt.wantSet {
					assert.Equal(t, []string{tt.wantHeader}, values, endpoint)
				}
			}
		})
	}
}
