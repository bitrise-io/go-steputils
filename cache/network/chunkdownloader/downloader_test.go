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
