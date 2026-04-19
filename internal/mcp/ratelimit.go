package mcp

import (
	"sync"
	"time"
)

// ring is a fixed-capacity circular buffer of timestamps used to track a
// single rate-limit key's recent allow() timestamps.
type ring struct {
	timestamps []time.Time
	head       int
	filled     bool
	capacity   int
}

func newRing(capacity int) *ring {
	if capacity < 1 {
		capacity = 1
	}
	return &ring{timestamps: make([]time.Time, capacity), capacity: capacity}
}

// liveTimestamps returns the timestamps newer than cutoff, in chronological
// order (oldest first).
func (r *ring) liveTimestamps(cutoff time.Time) []time.Time {
	n := r.head
	if r.filled {
		n = r.capacity
	}
	if n == 0 {
		return nil
	}
	out := make([]time.Time, 0, n)
	// Chronological walk: if filled, start at head (oldest); else start at 0.
	start := 0
	if r.filled {
		start = r.head
	}
	for i := 0; i < n; i++ {
		idx := (start + i) % r.capacity
		ts := r.timestamps[idx]
		if !ts.Before(cutoff) {
			out = append(out, ts)
		}
	}
	return out
}

func (r *ring) append(now time.Time) {
	r.timestamps[r.head] = now
	r.head = (r.head + 1) % r.capacity
	if r.head == 0 {
		r.filled = true
	}
}

// lastTimestamp returns the most recently appended timestamp, or zero if empty.
func (r *ring) lastTimestamp() time.Time {
	if !r.filled && r.head == 0 {
		return time.Time{}
	}
	idx := r.head - 1
	if idx < 0 {
		idx = r.capacity - 1
	}
	return r.timestamps[idx]
}

// Limiter is a sliding-window in-memory rate limiter keyed by arbitrary string.
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*ring
	once    sync.Once
}

// NewLimiter returns a fresh Limiter.
func NewLimiter() *Limiter {
	return &Limiter{windows: make(map[string]*ring)}
}

// Allow reports whether the key is under max events in the given window.
// On deny it returns the time until the oldest in-window event falls off.
func (l *Limiter) Allow(key string, max int, window time.Duration) (bool, time.Duration) {
	if max < 1 {
		return false, window
	}
	l.once.Do(l.startGC)

	l.mu.Lock()
	defer l.mu.Unlock()

	r, ok := l.windows[key]
	if !ok {
		r = newRing(max)
		l.windows[key] = r
	} else if r.capacity != max {
		// Capacity changed between calls; rebuild preserving live entries.
		live := r.liveTimestamps(time.Now().Add(-window))
		nr := newRing(max)
		for _, ts := range live {
			nr.append(ts)
		}
		r = nr
		l.windows[key] = r
	}

	now := time.Now()
	cutoff := now.Add(-window)
	live := r.liveTimestamps(cutoff)

	if len(live) >= max {
		oldest := live[0]
		retry := oldest.Add(window).Sub(now)
		if retry < 0 {
			retry = 0
		}
		return false, retry
	}

	// Rebuild ring with live entries + now.
	nr := newRing(max)
	for _, ts := range live {
		nr.append(ts)
	}
	nr.append(now)
	l.windows[key] = nr
	return true, 0
}

// startGC launches a background goroutine that periodically drops stale
// buckets (no activity in the last 10 minutes).
func (l *Limiter) startGC() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-10 * time.Minute)
			l.mu.Lock()
			for k, r := range l.windows {
				if r.lastTimestamp().Before(cutoff) {
					delete(l.windows, k)
				}
			}
			l.mu.Unlock()
		}
	}()
}
