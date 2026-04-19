package data

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// sampleEnvelope returns a minimal but parseable StoryPlotter envelope.
func sampleEnvelope(t *testing.T) []byte {
	t.Helper()
	b, err := Marshal(&Export{PlotList: []Plot{{Title: "Seed", FolderPath: "root"}}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDiskUserStore_LoadMissingUser(t *testing.T) {
	dir := t.TempDir()
	s := NewDiskUserStore(dir, "")
	exp, err := s.Load("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.PlotList) != 0 {
		t.Errorf("want empty, got %d plots", len(exp.PlotList))
	}
}

func TestDiskUserStore_LoadSharedMissing(t *testing.T) {
	dir := t.TempDir()
	s := NewDiskUserStore(dir, filepath.Join(dir, "shared.json"))
	exp, err := s.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.PlotList) != 0 {
		t.Errorf("want empty, got %d plots", len(exp.PlotList))
	}
}

func TestDiskUserStore_ReplaceLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewDiskUserStore(dir, "")
	raw := sampleEnvelope(t)
	if err := s.Replace("bob", raw); err != nil {
		t.Fatal(err)
	}
	exp, err := s.Load("bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.PlotList) != 1 || exp.PlotList[0].Title != "Seed" {
		t.Errorf("round trip mismatch: %+v", exp.PlotList)
	}
	// File should exist with 0600 perms.
	info, err := os.Stat(filepath.Join(dir, "bob", "storyplotter.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %v, want 0600", perm)
	}
}

func TestDiskUserStore_SaveSharedRejected(t *testing.T) {
	dir := t.TempDir()
	s := NewDiskUserStore(dir, filepath.Join(dir, "shared.json"))
	if err := s.Save("", &Export{}); err == nil {
		t.Errorf("expected error when saving with empty userID")
	}
	if err := s.Replace("", []byte("{}")); err == nil {
		t.Errorf("expected error when replacing with empty userID")
	}
}

func TestDiskUserStore_ReplaceRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	s := NewDiskUserStore(dir, "")
	if err := s.Replace("alice", []byte("not-json")); err == nil {
		t.Errorf("expected parse error for bad payload")
	}
}

func TestDiskUserStore_ConcurrentWritesSerialize(t *testing.T) {
	dir := t.TempDir()
	s := NewDiskUserStore(dir, "")
	raw := sampleEnvelope(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Replace("alice", raw); err != nil {
				t.Errorf("replace: %v", err)
			}
		}()
	}
	wg.Wait()

	// Final file must still parse — no corruption from interleaving.
	exp, err := s.Load("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.PlotList) != 1 {
		t.Errorf("after concurrent writes, want 1 plot, got %d", len(exp.PlotList))
	}
}

func TestDiskUserStore_SaveThenLoadWithExport(t *testing.T) {
	dir := t.TempDir()
	s := NewDiskUserStore(dir, "")
	exp := &Export{PlotList: []Plot{{Title: "Kept", FolderPath: "f"}}}
	if err := s.Save("carol", exp); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("carol")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.PlotList) != 1 || got.PlotList[0].Title != "Kept" {
		t.Errorf("mismatch: %+v", got.PlotList)
	}
}

func TestDiskUserStore_RawSharedFallback(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared.json")
	if err := os.WriteFile(shared, sampleEnvelope(t), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewDiskUserStore(dir, shared)
	b, err := s.Raw("")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Errorf("empty raw")
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := WriteAtomic(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"a":1}` {
		t.Errorf("content mismatch: %s", b)
	}
}
