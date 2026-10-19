package chunkdownloader

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/require"
)

func smallStreamDownloader() *Downloader {
	return New(Config{
		Concurrency:       3,
		MaxRetryPerChunk:  2,
		StreamChunkSize:   1000,
		StreamIdleTimeout: 100 * time.Millisecond,
	}, log.NewLogger())
}

// openStream opens url with small chunks and closes the stream when the test ends.
func openStream(t *testing.T, url string) (*Stream, error) {
	t.Helper()
	s, err := smallStreamDownloader().OpenStream(context.Background(), url)
	if err == nil {
		t.Cleanup(func() { require.NoError(t, s.Close()) })
	}
	return s, err
}

func randomData(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func rangeServer(t *testing.T, data []byte, override func(w http.ResponseWriter, r *http.Request) bool) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if override != nil && override(w, r) {
			return
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestOpenStream_readsChunksInOrder(t *testing.T) {
	data := randomData(10_007)
	url := rangeServer(t, data, nil)

	s, err := openStream(t, url)
	require.NoError(t, err)

	require.Equal(t, int64(len(data)), s.Size)
	got, err := io.ReadAll(s)
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func TestOpenStream_retriesStalledChunk(t *testing.T) {
	data := randomData(5_500)
	var mu sync.Mutex
	requests := map[string]int{}
	url := rangeServer(t, data, func(_ http.ResponseWriter, r *http.Request) bool {
		mu.Lock()
		requests[r.Header.Get("Range")]++
		stall := r.Header.Get("Range") == "bytes=3000-3999" && requests["bytes=3000-3999"] == 1
		mu.Unlock()
		if stall {
			<-r.Context().Done() // hang until the attempt is cancelled
		}
		return stall
	})

	s, err := openStream(t, url)
	require.NoError(t, err)

	got, err := io.ReadAll(s)
	require.NoError(t, err)
	require.Equal(t, data, got)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, requests["bytes=3000-3999"])
}

func TestOpenStream_failingChunkFailsRead(t *testing.T) {
	data := randomData(5_500)
	url := rangeServer(t, data, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Range") != "bytes=4000-4999" {
			return false
		}
		http.Error(w, "boom", http.StatusInternalServerError)
		return true
	})

	s, err := openStream(t, url)
	require.NoError(t, err)

	got, err := io.ReadAll(s)
	require.Error(t, err)
	require.Equal(t, data[:4000], got)
}

func TestOpenStream_failingFirstChunkFailsOpen(t *testing.T) {
	url := rangeServer(t, nil, func(w http.ResponseWriter, _ *http.Request) bool {
		http.Error(w, "boom", http.StatusInternalServerError)
		return true
	})

	_, err := openStream(t, url)
	require.Error(t, err)
}

func TestOpenStream_rangeIgnored(t *testing.T) {
	for _, tc := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{"fits in one chunk", 700, false},
		{"empty file", 0, false},
		{"larger than a chunk", 1500, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := randomData(tc.size)
			url := rangeServer(t, data, func(w http.ResponseWriter, _ *http.Request) bool {
				w.Header().Set("Content-Length", strconv.Itoa(len(data)))
				_, _ = w.Write(data)
				return true
			})

			s, err := openStream(t, url)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, err := io.ReadAll(s)
			require.NoError(t, err)
			require.Equal(t, data, got)
		})
	}
}
