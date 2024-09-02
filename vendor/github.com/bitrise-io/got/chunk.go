package got

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type OffsetWriter struct {
	io.WriterAt
	offset int64
}

func (dst *OffsetWriter) Write(b []byte) (n int, err error) {
	n, err = dst.WriteAt(b, dst.offset)
	dst.offset += int64(n)
	return
}

// Chunk represents the partial content range
type Chunk struct {
	Start, End uint64
}

type chunkStatistics struct {
	sum            time.Duration
	finishedChunks int
	mu             sync.Mutex
}

func (cs *chunkStatistics) update(d time.Duration) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.sum += d
	cs.finishedChunks++
}

func (cs *chunkStatistics) average() time.Duration {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.finishedChunks == 0 {
		return 0
	}
	return cs.sum / time.Duration(cs.finishedChunks)
}

func (cs *chunkStatistics) String() string {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var avg time.Duration
	if cs.finishedChunks > 0 {
		avg = cs.sum / time.Duration(cs.finishedChunks)
	}

	return fmt.Sprintf("[finishedChunks=%d][avg=%s]", cs.finishedChunks, avg.Round(time.Second))
}
