package chunkuploader

import (
	"sync"
	"time"
)

// Stats tracks upload performance metrics for hung detection and reporting.
type Stats struct {
	sum            time.Duration
	finishedChunks int64
	mu             sync.Mutex
}

// NewStats creates a new Stats instance.
func NewStats() *Stats {
	return &Stats{}
}

// Update records a successful chunk upload duration.
func (s *Stats) Update(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sum += d
	s.finishedChunks++
}

// Average returns the average upload duration for completed chunks.
func (s *Stats) Average() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finishedChunks == 0 {
		return 0
	}
	return s.sum / time.Duration(s.finishedChunks)
}

// FinishedCount returns the number of completed chunk uploads.
func (s *Stats) FinishedCount() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finishedChunks
}

// TotalDuration returns the sum of all upload durations.
func (s *Stats) TotalDuration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sum
}
