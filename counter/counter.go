package counter

import (
	"sync"
	"time"
)

// Key identifies a unique event source (e.g., "namespace/pod/reason").
type Key string

// SlidingWindow counts events within a rolling time window and fires
// when the count reaches the configured threshold.
type SlidingWindow struct {
	mu        sync.Mutex
	window    time.Duration
	threshold int
	events    map[Key][]time.Time
}

func NewSlidingWindow(window time.Duration, threshold int) *SlidingWindow {
	return &SlidingWindow{
		window:    window,
		threshold: threshold,
		events:    make(map[Key][]time.Time),
	}
}

// Record adds a timestamp for the key. Returns true if the threshold is met.
func (s *SlidingWindow) Record(key Key, at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := at.Add(-s.window)
	timestamps := s.events[key]

	// Drop timestamps outside the window
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	valid = append(valid, at)
	s.events[key] = valid

	return len(valid) >= s.threshold
}

// Prune removes stale keys. Call periodically to prevent memory leaks.
func (s *SlidingWindow) Prune() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-s.window)
	for key, timestamps := range s.events {
		valid := timestamps[:0]
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(s.events, key)
		} else {
			s.events[key] = valid
		}
	}
}
