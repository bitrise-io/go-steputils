package chunkuploader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestUploader_Upload_Success(t *testing.T) {
	// Create test server that returns ETags
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("ETag", fmt.Sprintf("\"etag-%d\"", count))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test chunks
	chunks := [][]byte{
		[]byte("chunk1-data"),
		[]byte("chunk2-data"),
		[]byte("chunk3-data"),
	}

	urls := make([]UploadURL, len(chunks))
	for i := range chunks {
		urls[i] = UploadURL{
			Method:  "PUT",
			URL:     server.URL,
			Headers: map[string]string{"Content-Type": "application/octet-stream"},
		}
	}

	provider := NewByteSliceChunkProvider(chunks)

	config := DefaultConfig()
	config.Concurrency = 2

	uploader := New(config)
	defer uploader.CloseIdleConnections()

	result, err := uploader.Upload(context.Background(), provider, urls)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if len(result.ETags) != len(chunks) {
		t.Fatalf("Expected %d ETags, got %d", len(chunks), len(result.ETags))
	}

	for i, etag := range result.ETags {
		if etag == "" {
			t.Errorf("ETag %d is empty", i)
		}
	}

	t.Logf("Upload completed with ETags: %v", result.ETags)
}

func TestUploader_Upload_Retry(t *testing.T) {
	// Create test server that fails first 2 attempts, then succeeds
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("temporary error"))
			return
		}
		w.Header().Set("ETag", "\"success-etag\"")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	chunks := [][]byte{[]byte("test-data")}
	urls := []UploadURL{{
		Method:  "PUT",
		URL:     server.URL,
		Headers: map[string]string{},
	}}

	provider := NewByteSliceChunkProvider(chunks)

	config := DefaultConfig()
	config.MaxRetryPerChunk = 3
	config.HungThreshold = 0 // Disable hung detection for this test

	uploader := New(config)
	defer uploader.CloseIdleConnections()

	result, err := uploader.Upload(context.Background(), provider, urls)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if result.ETags[0] != "\"success-etag\"" {
		t.Errorf("Expected success-etag, got %s", result.ETags[0])
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests (2 failures + 1 success), got %d", requestCount)
	}
}

func TestUploader_Upload_ContextCancellation(t *testing.T) {
	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Header().Set("ETag", "\"etag\"")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	chunks := [][]byte{[]byte("test-data")}
	urls := []UploadURL{{
		Method:  "PUT",
		URL:     server.URL,
		Headers: map[string]string{},
	}}

	provider := NewByteSliceChunkProvider(chunks)

	config := DefaultConfig()

	uploader := New(config)
	defer uploader.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := uploader.Upload(ctx, provider, urls)
	if err == nil {
		t.Fatal("Expected error due to context cancellation")
	}

	t.Logf("Got expected error: %v", err)
}

func TestUploader_UploadSingleChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "\"single-chunk-etag\"")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()

	uploader := New(config)
	defer uploader.CloseIdleConnections()

	url := UploadURL{
		Method:  "PUT",
		URL:     server.URL,
		Headers: map[string]string{},
	}

	etag, err := uploader.UploadSingleChunk(context.Background(), []byte("single-chunk-data"), url, 0, 1)
	if err != nil {
		t.Fatalf("UploadSingleChunk failed: %v", err)
	}

	if etag != "\"single-chunk-etag\"" {
		t.Errorf("Expected single-chunk-etag, got %s", etag)
	}
}

func TestStats(t *testing.T) {
	stats := NewStats()

	if stats.FinishedCount() != 0 {
		t.Errorf("Expected 0 finished, got %d", stats.FinishedCount())
	}

	if stats.Average() != 0 {
		t.Errorf("Expected 0 average, got %v", stats.Average())
	}

	stats.Update(100 * time.Millisecond)
	stats.Update(200 * time.Millisecond)
	stats.Update(300 * time.Millisecond)

	if stats.FinishedCount() != 3 {
		t.Errorf("Expected 3 finished, got %d", stats.FinishedCount())
	}

	expectedAvg := 200 * time.Millisecond
	if stats.Average() != expectedAvg {
		t.Errorf("Expected %v average, got %v", expectedAvg, stats.Average())
	}

	expectedTotal := 600 * time.Millisecond
	if stats.TotalDuration() != expectedTotal {
		t.Errorf("Expected %v total, got %v", expectedTotal, stats.TotalDuration())
	}
}

func TestOptimalChunkSizeBytes(t *testing.T) {
	tests := []struct {
		name        string
		totalSize   int64
		concurrency int
		minExpected int64
		maxExpected int64
	}{
		{
			name:        "small file",
			totalSize:   10 * 1024 * 1024, // 10MB
			concurrency: 4,
			minExpected: 8 * 1024 * 1024,  // min chunk size
			maxExpected: 10 * 1024 * 1024, // shouldn't exceed file size
		},
		{
			name:        "large file",
			totalSize:   1024 * 1024 * 1024, // 1GB
			concurrency: 10,
			minExpected: 8 * 1024 * 1024,   // min
			maxExpected: 100 * 1024 * 1024, // max
		},
		{
			name:        "very large file",
			totalSize:   10 * 1024 * 1024 * 1024, // 10GB
			concurrency: 20,
			minExpected: 8 * 1024 * 1024,   // min
			maxExpected: 100 * 1024 * 1024, // max
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OptimalChunkSizeBytes(tt.totalSize, tt.concurrency)
			if result < tt.minExpected {
				t.Errorf("Chunk size %d is below minimum %d", result, tt.minExpected)
			}
			if result > tt.maxExpected {
				t.Errorf("Chunk size %d exceeds maximum %d", result, tt.maxExpected)
			}
			t.Logf("File size: %dMB, Concurrency: %d, Chunk size: %dMB",
				tt.totalSize/(1024*1024), tt.concurrency, result/(1024*1024))
		})
	}
}

func TestDefaultConcurrency(t *testing.T) {
	c := DefaultConcurrency()
	if c < 2 {
		t.Errorf("Concurrency %d is below minimum 2", c)
	}
	if c > 20 {
		t.Errorf("Concurrency %d exceeds maximum 20", c)
	}
	t.Logf("Default concurrency: %d", c)
}
