package mcp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Token errors returned by Verify.
var (
	ErrTokenBadSignature = errors.New("token signature mismatch")
	ErrTokenExpired      = errors.New("token expired")
	ErrTokenReused       = errors.New("token already consumed")
	ErrTokenMalformed    = errors.New("token malformed")
)

// TokenSigner mints and verifies one-shot HMAC-SHA256 tokens for the
// /download endpoint. Consumed nonces are kept in an in-memory map until
// their expiry has passed, at which point a lazy sweep goroutine drops
// them.
type TokenSigner struct {
	key      []byte
	consumed sync.Map // nonce hex -> expUnix (int64)
}

// NewTokenSigner builds a signer with the given HMAC key. Panics if the key
// is shorter than 16 bytes. Starts a background goroutine that sweeps
// consumed nonces once a minute.
func NewTokenSigner(key []byte) *TokenSigner {
	if len(key) < 16 {
		panic(fmt.Sprintf("TokenSigner: key too short (%d bytes, need >=16)", len(key)))
	}
	ts := &TokenSigner{key: append([]byte(nil), key...)}
	go ts.sweepLoop()
	return ts
}

func (t *TokenSigner) sweepLoop() {
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	for range tick.C {
		now := time.Now().Unix()
		t.consumed.Range(func(k, v any) bool {
			if exp, ok := v.(int64); ok && exp < now {
				t.consumed.Delete(k)
			}
			return true
		})
	}
}

// Sign returns a token string for userID valid for ttl. Format:
//
//	base64url(userID).base64url(exp).base64url(nonce).base64url(mac)
//
// where the HMAC covers "userID|exp|hex(nonce)".
func (t *TokenSigner) Sign(userID string, ttl time.Duration) string {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		// crypto/rand failure is catastrophic; panic rather than ship a
		// predictable token.
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	exp := time.Now().Add(ttl).Unix()
	nonceHex := hex.EncodeToString(nonce[:])
	payload := userID + "|" + strconv.FormatInt(exp, 10) + "|" + nonceHex
	mac := hmacSHA256(t.key, []byte(payload))

	enc := base64.RawURLEncoding
	return strings.Join([]string{
		enc.EncodeToString([]byte(userID)),
		enc.EncodeToString([]byte(strconv.FormatInt(exp, 10))),
		enc.EncodeToString(nonce[:]),
		enc.EncodeToString(mac),
	}, ".")
}

// Verify parses and validates a token, returning the userID on success or
// one of ErrTokenMalformed / ErrTokenBadSignature / ErrTokenExpired /
// ErrTokenReused. A successful Verify atomically marks the token consumed;
// a second Verify of the same token returns ErrTokenReused.
func (t *TokenSigner) Verify(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return "", ErrTokenMalformed
	}
	enc := base64.RawURLEncoding
	uidB, err := enc.DecodeString(parts[0])
	if err != nil {
		return "", ErrTokenMalformed
	}
	expB, err := enc.DecodeString(parts[1])
	if err != nil {
		return "", ErrTokenMalformed
	}
	nonceB, err := enc.DecodeString(parts[2])
	if err != nil {
		return "", ErrTokenMalformed
	}
	macB, err := enc.DecodeString(parts[3])
	if err != nil {
		return "", ErrTokenMalformed
	}
	exp, err := strconv.ParseInt(string(expB), 10, 64)
	if err != nil {
		return "", ErrTokenMalformed
	}
	userID := string(uidB)
	nonceHex := hex.EncodeToString(nonceB)
	payload := userID + "|" + strconv.FormatInt(exp, 10) + "|" + nonceHex
	want := hmacSHA256(t.key, []byte(payload))
	if !hmac.Equal(macB, want) {
		return "", ErrTokenBadSignature
	}
	if time.Now().Unix() >= exp {
		return "", ErrTokenExpired
	}
	if _, loaded := t.consumed.LoadOrStore(nonceHex, exp); loaded {
		return "", ErrTokenReused
	}
	return userID, nil
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
