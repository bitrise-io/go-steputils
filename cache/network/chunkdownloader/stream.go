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
	// A chunk that is still trickling in never trips the idle timeout, so the one
	// Read is blocked on gets a second, racing request after this long.
	defaultStreamHedgeDelay = time.Second
	// A hedge fires only when Read waits this many times longer than its average
	// wait, so a healthy but bandwidth-limited download isn't hedged on every chunk.
	hedgeWaitFactor = 3
	// Sanity bound on the size a server reports, since chunk bookkeeping is
	// allocated upfront.
	maxStreamSize = 1 << 40
)

var (
	errIdleTimeout     = errors.New("no bytes received within the idle timeout")
	errRetryableStatus = errors.New("retryable status")
)

// Stream reads a remote file in order while fetching chunks ahead in parallel
// into memory. Memory is bounded by (Concurrency + 1) x StreamChunkSize, the +1
// being a hedged duplicate of the chunk Read is waiting on.
// Unlike DownloadFile, nothing is written to disk.
type Stream struct {
	// Size is the total size of the remote file.
	Size int64

	ctx    context.Context
	cancel context.CancelFunc
	f      streamFetcher
	offset int64
	// ETag of the first response; later chunks are requested with If-Match, so a
	// file replaced mid-stream (even with the same size) fails instead of mixing.
	etag    string
	results []chan streamResult
	// Per-chunk contexts, so the losing attempt of a hedged chunk can be stopped.
	chunkCtx    []context.Context
	chunkCancel []context.CancelFunc
	slots       chan struct{}
	next        int
	held        bool
	cur         []byte
	err         error
	avgWait     time.Duration
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
// once it returns. The first chunk is retried but not hedged: a caller that has
// not sent anything downstream yet has no idle timer running. The caller must
// Close the stream.
func (d *Downloader) OpenStream(ctx context.Context, url string) (*Stream, error) {
	f := d.streamFetcher(url)
	ctx, cancel := context.WithCancel(ctx)

	first, size, etag, err := f.fetch(ctx, 0, f.chunkSize-1, "")
	if err == nil && size > maxStreamSize {
		err = fmt.Errorf("reported file size %d exceeds %d", size, int64(maxStreamSize))
	}
	if err != nil {
		cancel()
		f.closeIdle()
		return nil, err
	}

	offset := int64(len(first))
	n := int((size - offset + f.chunkSize - 1) / f.chunkSize)
	s := &Stream{
		Size:        size,
		ctx:         ctx,
		cancel:      cancel,
		f:           f,
		offset:      offset,
		etag:        etag,
		results:     make([]chan streamResult, n),
		chunkCtx:    make([]context.Context, n),
		chunkCancel: make([]context.CancelFunc, n),
		slots:       make(chan struct{}, f.concurrency),
		held:        true,
		cur:         first,
	}
	s.slots <- struct{}{} // the first chunk counts against the memory bound too
	for i := range s.results {
		// Room for the original and a hedged attempt, so neither sender blocks.
		s.results[i] = make(chan streamResult, 2)
		// Created upfront: a hedge can start before the scheduler reaches chunk i.
		s.chunkCtx[i], s.chunkCancel[i] = context.WithCancel(ctx)
	}

	// A slot is released only once its chunk is fully read, which bounds memory.
	go func() {
		for i := 0; i < n; i++ {
			select {
			case s.slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			go s.fetchChunk(i)
		}
	}()

	return s, nil
}

func (s *Stream) chunkRange(i int) (int64, int64) {
	start := s.offset + int64(i)*s.f.chunkSize
	return start, min(start+s.f.chunkSize, s.Size) - 1
}

// fetchChunk fetches chunk i into its result channel. It runs once per chunk,
// plus once more if the chunk gets hedged.
func (s *Stream) fetchChunk(i int) {
	start, end := s.chunkRange(i)
	buf, total, _, err := s.f.fetch(s.chunkCtx[i], start, end, s.etag)
	if err == nil && total != s.Size {
		err = fmt.Errorf("bytes %d-%d: file size changed from %d to %d mid-stream", start, end, s.Size, total)
	}
	if err == nil && int64(len(buf)) != end-start+1 {
		err = fmt.Errorf("bytes %d-%d: got %d bytes", start, end, len(buf))
	}
	s.results[i] <- streamResult{buf, err}
}

// await waits for chunk i. Once the wait is well above the usual one, a second
// request for the same range races the first; the first success wins.
func (s *Stream) await(i int) (streamResult, error) {
	var hedgeC <-chan time.Time
	if s.f.hedgeDelay > 0 {
		hedge := time.NewTimer(max(s.f.hedgeDelay, hedgeWaitFactor*s.avgWait))
		defer hedge.Stop()
		hedgeC = hedge.C
	}
	started := time.Now()
	pending := 1
	var firstErr error
	for {
		select {
		case res := <-s.results[i]:
			pending--
			if res.err != nil && pending > 0 {
				firstErr = res.err // the other attempt may still succeed
				continue
			}
			if res.err != nil && firstErr != nil {
				res.err = firstErr
			}
			if pending > 0 {
				// Drop the loser's result once it lands, or its buffer would stay
				// in the channel for the stream's lifetime.
				go func(ch chan streamResult) { <-ch }(s.results[i])
			}
			s.recordWait(time.Since(started))
			return res, nil
		case <-hedgeC:
			start, end := s.chunkRange(i)
			s.f.d.logger.Warnf("Chunk bytes %d-%d not ready after %s, hedging with a second request",
				start, end, time.Since(started).Round(time.Millisecond))
			pending++
			go s.fetchChunk(i)
		case <-s.ctx.Done():
			return streamResult{}, s.ctx.Err()
		}
	}
}

// recordWait keeps an exponential moving average of how long Read waits per chunk.
func (s *Stream) recordWait(d time.Duration) {
	if s.avgWait == 0 {
		s.avgWait = d
		return
	}
	s.avgWait = (4*s.avgWait + d) / 5
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
		// Chunks after a cancel may never be scheduled, so await doesn't wait for them.
		res, err := s.await(s.next)
		if err != nil {
			s.err = err
			return 0, s.err
		}
		s.chunkCancel[s.next]() // stops the losing attempt of a hedged chunk
		s.next++
		if res.err != nil {
			s.err = res.err
			s.cancel() // nothing after a failed chunk will be read
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
	s.f.closeIdle()
	return nil
}

type streamFetcher struct {
	d           *Downloader
	client      *http.Client
	ownsClient  bool
	url         string
	chunkSize   int64
	concurrency int
	tries       int
	idle        time.Duration
	hedgeDelay  time.Duration
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
		hedgeDelay:  d.config.StreamHedgeDelay,
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
	if f.hedgeDelay == 0 {
		f.hedgeDelay = defaultStreamHedgeDelay
	}
	if f.client == nil {
		// A private client: tuning got.DefaultClient in place would race with
		// requests of other downloads sharing it.
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConnsPerHost = f.concurrency
		f.client = &http.Client{Transport: t}
		f.ownsClient = true
		applyTransportTuning(f.client)
	}
	return f
}

// closeIdle releases the private client's idle connections; a caller's client is left alone.
func (f streamFetcher) closeIdle() {
	if f.ownsClient {
		f.client.CloseIdleConnections()
	}
}

// fetch returns bytes [start,end], the file's total size and its ETag, retrying
// failed or stalled attempts. A non-empty ifMatch pins the file version.
func (f streamFetcher) fetch(ctx context.Context, start, end int64, ifMatch string) ([]byte, int64, string, error) {
	var err error
	for attempt := 1; attempt <= f.tries; attempt++ {
		var buf []byte
		var size int64
		var etag string
		if buf, size, etag, err = f.fetchOnce(ctx, start, end, ifMatch); err == nil {
			return buf, size, etag, nil
		}
		if ctx.Err() != nil {
			return nil, 0, "", fmt.Errorf("bytes %d-%d: %w", start, end, ctx.Err())
		}
		var perm permanentError
		if errors.As(err, &perm) {
			return nil, 0, "", fmt.Errorf("bytes %d-%d: %w", start, end, err)
		}
		f.d.logger.Warnf("Chunk bytes %d-%d attempt %d/%d failed: %s", start, end, attempt, f.tries, err)
		if errors.Is(err, errRetryableStatus) && attempt < f.tries {
			// Short, so a throttled backend gets a breather without eating the
			// downstream client's idle budget; stall retries stay immediate.
			select {
			case <-time.After(min(time.Duration(attempt)*250*time.Millisecond, time.Second)):
			case <-ctx.Done():
			}
		}
	}
	return nil, 0, "", fmt.Errorf("bytes %d-%d after %d tries: %w", start, end, f.tries, err)
}

func (f streamFetcher) fetchOnce(ctx context.Context, start, end int64, ifMatch string) ([]byte, int64, string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	idle := time.AfterFunc(f.idle, func() { cancel(errIdleTimeout) })
	defer idle.Stop()

	buf, size, etag, err := f.get(ctx, start, end, ifMatch, idle)
	if err != nil && errors.Is(context.Cause(ctx), errIdleTimeout) {
		// Otherwise it surfaces as a bare "context canceled".
		err = fmt.Errorf("%w (%s): %w", errIdleTimeout, f.idle, err)
	}
	return buf, size, etag, err
}

func (f streamFetcher) get(ctx context.Context, start, end int64, ifMatch string, idle *time.Timer) ([]byte, int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var size, want int64
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		var first, last int64
		cr := resp.Header.Get("Content-Range")
		if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &first, &last, &size); err != nil {
			// e.g. an unknown total ("bytes 0-999/*"): the same answer every time.
			return nil, 0, "", permanentError{fmt.Errorf("unparsable Content-Range %q", cr)}
		}
		if first != start || last < first || last > end || size <= last {
			return nil, 0, "", fmt.Errorf("unexpected Content-Range %q for bytes %d-%d", cr, start, end)
		}
		want = last - first + 1
	case resp.StatusCode == http.StatusOK && start == 0:
		// Range ignored (e.g. an empty file): fine as long as it fits in one chunk.
		size, want = resp.ContentLength, resp.ContentLength
		if want < 0 || want > end-start+1 {
			return nil, 0, "", fmt.Errorf("range ignored and Content-Length %d does not fit bytes %d-%d", want, start, end)
		}
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && start == 0 &&
		resp.Header.Get("Content-Range") == "bytes */0":
		// How S3/GCS-style backends answer a Range request on an empty file.
		return []byte{}, 0, resp.Header.Get("ETag"), nil
	case resp.StatusCode == http.StatusPreconditionFailed && ifMatch != "":
		return nil, 0, "", permanentError{fmt.Errorf("file changed mid-stream (If-Match %s)", ifMatch)}
	case resp.StatusCode >= 400 && resp.StatusCode < 500 &&
		resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests:
		// e.g. an expired signed URL: retrying can't help.
		return nil, 0, "", permanentError{fmt.Errorf("unexpected status %d", resp.StatusCode)}
	default:
		return nil, 0, "", fmt.Errorf("unexpected status %d: %w", resp.StatusCode, errRetryableStatus)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != want {
		return nil, 0, "", fmt.Errorf("body length %d does not match %d range bytes", resp.ContentLength, want)
	}

	buf := make([]byte, want)
	for off := 0; off < len(buf); {
		idle.Reset(f.idle)
		n, err := resp.Body.Read(buf[off:])
		off += n
		if err != nil && (err != io.EOF || off < len(buf)) {
			return nil, 0, "", fmt.Errorf("read body at %d/%d: %w", off, len(buf), err)
		}
	}
	return buf, size, resp.Header.Get("ETag"), nil
}
