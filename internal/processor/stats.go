package processor

import (
	"sync"
)

// Stats tracks basic service metrics for logging purposes
type Stats struct {
	mu                   sync.RWMutex
	TotalEventsForwarded int64
	TotalAPIRequests     int64
	FailedAPIRequests    int64
}

// NewStats creates a new stats tracker
func NewStats() *Stats {
	return &Stats{}
}

// IncrementEventsForwarded adds to the events counter
func (s *Stats) IncrementEventsForwarded(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalEventsForwarded += count
}

// IncrementAPIRequests increments the API request counter
func (s *Stats) IncrementAPIRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalAPIRequests++
}

// IncrementFailedAPIRequests increments the failed API request counter
func (s *Stats) IncrementFailedAPIRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailedAPIRequests++
}

// GetTotalEvents returns the total events forwarded (thread-safe)
func (s *Stats) GetTotalEvents() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TotalEventsForwarded
}

// GetTotalAPIRequests returns the total API requests (thread-safe)
func (s *Stats) GetTotalAPIRequests() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TotalAPIRequests
}

// GetFailedAPIRequests returns the failed API requests (thread-safe)
func (s *Stats) GetFailedAPIRequests() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.FailedAPIRequests
}
