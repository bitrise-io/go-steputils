package chunkdownloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/require"
)

// got dereferences dl.Logger without a nil check inside its chunk-download
// goroutine, so DownloadFile must set it. Before the fix this panicked with a
// nil pointer dereference on rangeable (chunked) downloads.
func TestDownloadFile_rangeableDownloadDoesNotPanic(t *testing.T) {
	content := strings.Repeat("bitrise-build-cache-", 30000) // ~600 KB, chunked

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "cache.tzst", time.Time{}, strings.NewReader(content))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.tzst")
	cfg := DefaultConfig()
	cfg.Concurrency = 4

	err := New(cfg, log.NewLogger()).DownloadFile(context.Background(), srv.URL, dest)
	require.NoError(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}

// A transport cloned from http.DefaultTransport keeps offering h2 in ALPN after
// applyTransportTuning turns HTTP/2 off. A server that picks h2 then answers with
// HTTP/2 frames, and every request fails with "malformed HTTP response".
func TestApplyTransportTuning_http2CapableServer(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	// Clone sets the copy's TLSClientConfig up for HTTP/2 (NextProtos with h2); only
	// trust the test server on top of it.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.RootCAs = server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	client := &http.Client{Transport: transport}
	applyTransportTuning(client)

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "HTTP/1.1", resp.Proto, "HTTP/2 is off unless forced")
}
