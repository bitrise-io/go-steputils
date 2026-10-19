package chunkdownloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
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

	ctx     context.Context
	cancel  context.CancelFunc
	client  *http.Client
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

// permanentError marks a chunk failure that is not worth retrying.
type permanentError struct{ error }

func (e permanentError) Unwrap() error { return e.error }

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
		ctx:     ctx,
		cancel:  cancel,
		client:  f.client,
		results: make([]chan streamResult, n),
		slots:   make(chan struct{}, f.concurrency),
		held:    true,
		cur:     first,
	}
	s.slots <- struct{}{} // the first chunk counts against the memory bound too
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
				buf, total, err := f.fetch(ctx, start, end)
				if err == nil && total != size {
					err = fmt.Errorf("bytes %d-%d: file size changed from %d to %d mid-stream", start, end, size, total)
				}
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
		var res streamResult
		select {
		case res = <-s.results[s.next]:
		case <-s.ctx.Done():
			// Chunks after a cancel may never be scheduled, so don't wait for them.
			s.err = s.ctx.Err()
			return 0, s.err
		}
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
	s.client.CloseIdleConnections()
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
	f := streamFetcher{
		d:           d,
		client:      d.config.HTTPClient,
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
	if f.client == nil {
		// A private client: tuning got.DefaultClient in place would race with
		// requests of other downloads sharing it.
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConnsPerHost = f.concurrency
		f.client = &http.Client{Transport: t}
		applyTransportTuning(f.client)
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
		var perm permanentError
		if errors.As(err, &perm) {
			return nil, 0, fmt.Errorf("bytes %d-%d: %w", start, end, err)
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

	var size, want int64
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		var first, last int64
		cr := resp.Header.Get("Content-Range")
		if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &first, &last, &size); err != nil ||
			first != start || last < first || last > end || size <= last {
			return nil, 0, fmt.Errorf("unexpected Content-Range %q for bytes %d-%d", cr, start, end)
		}
		want = last - first + 1
	case resp.StatusCode == http.StatusOK && start == 0:
		// Range ignored (e.g. an empty file): fine as long as it fits in one chunk.
		size, want = resp.ContentLength, resp.ContentLength
		if want < 0 || want > end-start+1 {
			return nil, 0, fmt.Errorf("range ignored and Content-Length %d does not fit bytes %d-%d", want, start, end)
		}
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && start == 0 &&
		resp.Header.Get("Content-Range") == "bytes */0":
		// How S3/GCS-style backends answer a Range request on an empty file.
		return []byte{}, 0, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500 &&
		resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests:
		// e.g. an expired signed URL: retrying can't help.
		return nil, 0, permanentError{fmt.Errorf("unexpected status %d", resp.StatusCode)}
	default:
		return nil, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != want {
		return nil, 0, fmt.Errorf("body length %d does not match %d range bytes", resp.ContentLength, want)
	}

	buf := make([]byte, want)
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
