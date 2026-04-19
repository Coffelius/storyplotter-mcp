package mcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
)

// rawStore is a UserStore that serves raw bytes for a single user, used to
// exercise the /download endpoint end-to-end.
type rawStore struct {
	userID string
	bytes  []byte
}

func (r *rawStore) Load(string) (*data.Export, error) { return &data.Export{}, nil }
func (r *rawStore) Save(string, *data.Export) error   { return nil }
func (r *rawStore) Raw(uid string) ([]byte, error) {
	if uid == r.userID {
		return r.bytes, nil
	}
	return nil, nil
}
func (r *rawStore) Replace(string, []byte) error { return nil }

func TestDownload_HappyPath(t *testing.T) {
	body := []byte(`{"memoList":"[]","tagColorMap":"{}","plotList":"[]","allFolderList":"[]"}`)
	store := &rawStore{userID: "alice", bytes: body}
	signer := NewTokenSigner(testKey())
	s := NewServer(store, signer, "https://example.test")
	ts := httptest.NewServer(s.Handler(HTTPConfig{Bearer: "secret"}))
	defer ts.Close()

	token := signer.Sign("alice", 5*time.Minute)

	resp, err := http.Get(ts.URL + "/download?t=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, "storyplotter-") || !strings.Contains(cd, ".json") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != string(body) {
		t.Errorf("body mismatch:\n got %q\nwant %q", got, body)
	}

	// Second GET → token reused → 410 Gone.
	resp2, err := http.Get(ts.URL + "/download?t=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusGone {
		t.Errorf("second GET status = %d, want 410", resp2.StatusCode)
	}
}

func TestDownload_ForgedToken(t *testing.T) {
	store := &rawStore{userID: "alice", bytes: []byte(`{"memoList":"[]","tagColorMap":"{}","plotList":"[]","allFolderList":"[]"}`)}
	signer := NewTokenSigner(testKey())
	s := NewServer(store, signer, "")
	ts := httptest.NewServer(s.Handler(HTTPConfig{Bearer: "secret"}))
	defer ts.Close()

	// Sign with a different key and submit.
	other := NewTokenSigner([]byte("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"))
	forged := other.Sign("alice", time.Minute)

	resp, err := http.Get(ts.URL + "/download?t=" + forged)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDownload_MissingToken(t *testing.T) {
	store := &rawStore{}
	signer := NewTokenSigner(testKey())
	s := NewServer(store, signer, "")
	ts := httptest.NewServer(s.Handler(HTTPConfig{Bearer: "secret"}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/download")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDownload_NoDataForUser(t *testing.T) {
	store := &rawStore{userID: "someoneelse", bytes: []byte("x")}
	signer := NewTokenSigner(testKey())
	s := NewServer(store, signer, "")
	ts := httptest.NewServer(s.Handler(HTTPConfig{Bearer: "secret"}))
	defer ts.Close()

	token := signer.Sign("alice", time.Minute)
	resp, err := http.Get(ts.URL + "/download?t=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
