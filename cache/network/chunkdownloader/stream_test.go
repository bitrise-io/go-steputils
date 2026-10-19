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
		StreamHedgeDelay:  300 * time.Millisecond,
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

// A chunk that trickles in never trips the idle timeout; the hedged request must win.
func TestOpenStream_hedgesTricklingChunk(t *testing.T) {
	data := randomData(5_500)
	var mu sync.Mutex
	requests := map[string]int{}
	url := rangeServer(t, data, func(w http.ResponseWriter, r *http.Request) bool {
		mu.Lock()
		requests[r.Header.Get("Range")]++
		trickle := r.Header.Get("Range") == "bytes=3000-3999" && requests["bytes=3000-3999"] == 1
		mu.Unlock()
		if !trickle {
			return false
		}
		w.Header().Set("Content-Range", "bytes 3000-3999/5500")
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusPartialContent)
		for _, b := range data[3000:4000] { // 1 byte per 50ms: 50s for the chunk
			select {
			case <-r.Context().Done(): // the losing attempt is cancelled
				return true
			case <-time.After(50 * time.Millisecond):
			}
			_, _ = w.Write([]byte{b})
			w.(http.Flusher).Flush()
		}
		return true
	})

	start := time.Now()
	s, err := openStream(t, url)
	require.NoError(t, err)
	got, err := io.ReadAll(s)
	require.NoError(t, err)
	require.Equal(t, data, got)
	require.Less(t, time.Since(start), 5*time.Second)
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

func TestOpenStream_cancelWithFullWindowDoesNotHang(t *testing.T) {
	data := randomData(10_007)
	url := rangeServer(t, data, nil)

	ctx, cancel := context.WithCancel(context.Background())
	s, err := smallStreamDownloader().OpenStream(ctx, url)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	time.Sleep(100 * time.Millisecond) // let the prefetch window fill
	cancel()

	done := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(s)
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Read hung after cancel")
	}
}

func TestOpenStream_rejectsWrongContentRange(t *testing.T) {
	data := randomData(5_500)
	url := rangeServer(t, data, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Range") != "bytes=0-999" {
			return false
		}
		w.Header().Set("Content-Range", "bytes 1000-1999/5500")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[1000:2000])
		return true
	})

	_, err := openStream(t, url)
	require.Error(t, err)
}

// unknownLength strips Content-Length and flushes every write, forcing a chunked response.
type unknownLength struct{ http.ResponseWriter }

func (w unknownLength) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w unknownLength) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.ResponseWriter.(http.Flusher).Flush()
	return n, err
}

func TestOpenStream_chunkedPartialContent(t *testing.T) {
	data := randomData(5_500)
	url := rangeServer(t, data, func(w http.ResponseWriter, r *http.Request) bool {
		http.ServeContent(unknownLength{w}, r, "", time.Time{}, bytes.NewReader(data))
		return true
	})

	s, err := openStream(t, url)
	require.NoError(t, err)
	got, err := io.ReadAll(s)
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func TestOpenStream_emptyFileRangeNotSatisfiable(t *testing.T) {
	url := rangeServer(t, nil, func(w http.ResponseWriter, _ *http.Request) bool {
		w.Header().Set("Content-Range", "bytes */0")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return true
	})

	s, err := openStream(t, url)
	require.NoError(t, err)
	require.Equal(t, int64(0), s.Size)
	got, err := io.ReadAll(s)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestOpenStream_clientErrorIsNotRetried(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	url := rangeServer(t, nil, func(w http.ResponseWriter, _ *http.Request) bool {
		mu.Lock()
		requests++
		mu.Unlock()
		http.Error(w, "expired", http.StatusForbidden)
		return true
	})

	_, err := openStream(t, url)
	require.Error(t, err)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, requests)
}

func TestOpenStream_rejectsSizeChangeMidStream(t *testing.T) {
	data := randomData(5_500)
	url := rangeServer(t, data, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Range") != "bytes=3000-3999" {
			return false
		}
		w.Header().Set("Content-Range", "bytes 3000-3999/9999")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[3000:4000])
		return true
	})

	s, err := openStream(t, url)
	require.NoError(t, err)
	got, err := io.ReadAll(s)
	require.ErrorContains(t, err, "size changed")
	require.Equal(t, data[:3000], got)
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
