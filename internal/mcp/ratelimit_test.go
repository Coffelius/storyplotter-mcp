package mcp

import (
	"testing"
	"time"
)

func TestLimiterAllowSlidingWindow(t *testing.T) {
	l := NewLimiter()
	window := 1 * time.Second
	const max = 3

	for i := 0; i < max; i++ {
		ok, retry := l.Allow("k", max, window)
		if !ok {
			t.Fatalf("call %d: expected allowed, got deny (retry=%v)", i+1, retry)
		}
	}

	ok, retry := l.Allow("k", max, window)
	if ok {
		t.Fatalf("4th call: expected deny, got allowed")
	}
	if retry <= 0 {
		t.Errorf("retryAfter = %v, want > 0", retry)
	}
	if retry > window {
		t.Errorf("retryAfter = %v, want <= %v", retry, window)
	}

	// After the window expires the bucket should drain.
	time.Sleep(1100 * time.Millisecond)
	ok, _ = l.Allow("k", max, window)
	if !ok {
		t.Errorf("post-window call: expected allowed, got deny")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	l := NewLimiter()
	window := 1 * time.Second

	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("a", 2, window); !ok {
			t.Fatalf("a: call %d denied", i+1)
		}
	}
	if ok, _ := l.Allow("a", 2, window); ok {
		t.Fatalf("a: 3rd call unexpectedly allowed")
	}
	if ok, _ := l.Allow("b", 2, window); !ok {
		t.Errorf("b should have its own bucket")
	}
}
