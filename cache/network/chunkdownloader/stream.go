package chunkdownloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bitrise-io/got"
)

const (
	defaultStreamChunkSize   = 16 << 20
	defaultStreamConcurrency = 8
	// Stays under @actions/cache's 5s socket idle timeout, so a stalled chunk is
	// retried before a client that reads the stream gives up.
	defaultStreamIdleTimeout = 2 * time.Second
)

// Stream reads a remote file in order while fetching chunks ahead in parallel
// into memory. Memory is bounded by Concurrency x StreamChunkSize.
// Unlike DownloadFile, nothing is written to disk.
type Stream struct {
	// Size is the total size of the remote file.
	Size int64

	cancel  context.CancelFunc
	results []chan streamResult
	slots   chan struct{}
	next    int
	held    bool
	cur     []byte
	err     error
}

type streamResult struct {
	buf []byte
	err error
}

// OpenStream fetches the first chunk of url and starts fetching the rest in the
// background. An error here means no byte has been read yet, and Size is known
// once it returns. The caller must Close the stream.
func (d *Downloader) OpenStream(ctx context.Context, url string) (*Stream, error) {
	f := d.streamFetcher(url)
	ctx, cancel := context.WithCancel(ctx)

	first, size, err := f.fetch(ctx, 0, f.chunkSize-1)
	if err != nil {
		cancel()
		return nil, err
	}

	offset := int64(len(first))
	n := int((size - offset + f.chunkSize - 1) / f.chunkSize)
	s := &Stream{
		Size:    size,
		cancel:  cancel,
		results: make([]chan streamResult, n),
		slots:   make(chan struct{}, f.concurrency),
		cur:     first,
	}
	for i := range s.results {
		s.results[i] = make(chan streamResult, 1)
	}

	// A slot is released only once its chunk is fully read, which bounds memory.
	go func() {
		for i := 0; i < n; i++ {
			select {
			case s.slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			go func(i int) {
				start := offset + int64(i)*f.chunkSize
				end := min(start+f.chunkSize, size) - 1
				buf, _, err := f.fetch(ctx, start, end)
				if err == nil && int64(len(buf)) != end-start+1 {
					err = fmt.Errorf("bytes %d-%d: got %d bytes", start, end, len(buf))
				}
				s.results[i] <- streamResult{buf, err}
			}(i)
		}
	}()

	return s, nil
}

// Read implements io.Reader.
func (s *Stream) Read(p []byte) (int, error) {
	for len(s.cur) == 0 {
		if s.err != nil {
			return 0, s.err
		}
		if s.held {
			<-s.slots
			s.held = false
		}
		if s.next == len(s.results) {
			return 0, io.EOF
		}
		res := <-s.results[s.next]
		s.next++
		if res.err != nil {
			s.err = res.err
			return 0, s.err
		}
		s.cur, s.held = res.buf, true
	}
	n := copy(p, s.cur)
	s.cur = s.cur[n:]
	return n, nil
}

// Close stops all in-flight chunk fetches.
func (s *Stream) Close() error {
	s.cancel()
	return nil
}

type streamFetcher struct {
	d           *Downloader
	client      *http.Client
	url         string
	chunkSize   int64
	concurrency int
	tries       int
	idle        time.Duration
}

func (d *Downloader) streamFetcher(url string) streamFetcher {
	client := d.config.HTTPClient
	if client == nil {
		client = got.DefaultClient
	}
	applyTransportTuning(client)

	f := streamFetcher{
		d:           d,
		client:      client,
		url:         url,
		chunkSize:   d.config.StreamChunkSize,
		concurrency: int(d.config.Concurrency),
		tries:       1 + max(d.config.MaxRetryPerChunk, 0),
		idle:        d.config.StreamIdleTimeout,
	}
	if f.chunkSize <= 0 {
		f.chunkSize = defaultStreamChunkSize
	}
	if f.concurrency <= 0 {
		f.concurrency = defaultStreamConcurrency
	}
	if f.idle <= 0 {
		f.idle = defaultStreamIdleTimeout
	}
	return f
}

// fetch returns bytes [start,end] and the file's total size, retrying failed or stalled attempts.
func (f streamFetcher) fetch(ctx context.Context, start, end int64) ([]byte, int64, error) {
	var err error
	for attempt := 1; attempt <= f.tries; attempt++ {
		var buf []byte
		var size int64
		if buf, size, err = f.fetchOnce(ctx, start, end); err == nil {
			return buf, size, nil
		}
		if ctx.Err() != nil {
			return nil, 0, fmt.Errorf("bytes %d-%d: %w", start, end, ctx.Err())
		}
		f.d.logger.Warnf("Chunk bytes %d-%d attempt %d/%d failed: %s", start, end, attempt, f.tries, err)
	}
	return nil, 0, fmt.Errorf("bytes %d-%d after %d tries: %w", start, end, f.tries, err)
}

func (f streamFetcher) fetchOnce(ctx context.Context, start, end int64) ([]byte, int64, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	idle := time.AfterFunc(f.idle, cancel)
	defer idle.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var size int64
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		_, total, _ := strings.Cut(resp.Header.Get("Content-Range"), "/")
		if size, err = strconv.ParseInt(total, 10, 64); err != nil {
			return nil, 0, fmt.Errorf("bad Content-Range %q", resp.Header.Get("Content-Range"))
		}
	case resp.StatusCode == http.StatusOK && start == 0:
		// Range ignored (e.g. an empty file): fine as long as it fits in one chunk.
		size = resp.ContentLength
	default:
		return nil, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if resp.ContentLength < 0 || resp.ContentLength > end-start+1 {
		return nil, 0, fmt.Errorf("unexpected Content-Length %d for bytes %d-%d", resp.ContentLength, start, end)
	}

	buf := make([]byte, resp.ContentLength)
	for off := 0; off < len(buf); {
		idle.Reset(f.idle)
		n, err := resp.Body.Read(buf[off:])
		off += n
		if err != nil && (err != io.EOF || off < len(buf)) {
			return nil, 0, fmt.Errorf("read body at %d/%d: %w", off, len(buf), err)
		}
	}
	return buf, size, nil
}
