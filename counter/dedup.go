package counter

import (
	"sync"
	"time"
)

// Deduplicator suppresses repeated notifications within a cooldown period.
type Deduplicator struct {
	mu       sync.Mutex
	cooldown time.Duration
	last     map[Key]time.Time
}

func NewDeduplicator(cooldown time.Duration) *Deduplicator {
	return &Deduplicator{
		cooldown: cooldown,
		last:     make(map[Key]time.Time),
	}
}

// ShouldNotify returns true if this key hasn't been notified within the cooldown.
// If true, marks the key as notified.
func (d *Deduplicator) ShouldNotify(key Key) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if lastTime, ok := d.last[key]; ok {
		if now.Sub(lastTime) < d.cooldown {
			return false
		}
	}
	d.last[key] = now
	return true
}

// Prune removes expired entries.
func (d *Deduplicator) Prune() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for key, t := range d.last {
		if now.Sub(t) >= d.cooldown {
			delete(d.last, key)
		}
	}
}
