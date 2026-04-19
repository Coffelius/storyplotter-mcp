package mcp

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func testKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef") // 32 bytes
}

func TestTokenSignVerifyRoundTrip(t *testing.T) {
	ts := NewTokenSigner(testKey())
	token := ts.Sign("alice", time.Minute)
	uid, err := ts.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if uid != "alice" {
		t.Errorf("uid = %q, want alice", uid)
	}
}

func TestTokenTampered(t *testing.T) {
	ts := NewTokenSigner(testKey())
	token := ts.Sign("alice", time.Minute)
	// Flip a character in the MAC segment (last one).
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		t.Fatalf("token format unexpected: %q", token)
	}
	macBytes := []byte(parts[3])
	// Swap first char to something different but still base64url-safe.
	if macBytes[0] == 'A' {
		macBytes[0] = 'B'
	} else {
		macBytes[0] = 'A'
	}
	parts[3] = string(macBytes)
	tampered := strings.Join(parts, ".")
	_, err := ts.Verify(tampered)
	if !errors.Is(err, ErrTokenBadSignature) {
		t.Errorf("err = %v, want ErrTokenBadSignature", err)
	}
}

func TestTokenExpired(t *testing.T) {
	ts := NewTokenSigner(testKey())
	// ttl=0 → exp is now; sleep past it.
	token := ts.Sign("alice", 0)
	time.Sleep(1100 * time.Millisecond) // exp uses second resolution
	_, err := ts.Verify(token)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestTokenReused(t *testing.T) {
	ts := NewTokenSigner(testKey())
	token := ts.Sign("alice", time.Minute)
	if _, err := ts.Verify(token); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	_, err := ts.Verify(token)
	if !errors.Is(err, ErrTokenReused) {
		t.Errorf("err = %v, want ErrTokenReused", err)
	}
}

func TestTokenMalformed(t *testing.T) {
	ts := NewTokenSigner(testKey())
	_, err := ts.Verify("not-a-token")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v, want ErrTokenMalformed", err)
	}
}

func TestNewTokenSignerShortKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for short key")
		}
	}()
	_ = NewTokenSigner([]byte("short"))
}
