package data

import (
	"container/list"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrSharedReadOnly is returned when a write is attempted against the anon
// / shared corpus (userID == "").
var ErrSharedReadOnly = errors.New("cannot write to shared corpus (empty user id)")

// DiskUserStore is a UserStore implementation backed by a per-user JSON file
// under baseDir, with an optional shared legacy path used when userID is
// empty. Reads are cached (simple LRU), writes are atomic and serialised
// per-user via a mutex map.
type DiskUserStore struct {
	baseDir    string
	sharedPath string

	cacheMu sync.Mutex
	cache   *lruCache // key: userID ("" == shared)

	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

// NewDiskUserStore constructs a DiskUserStore. baseDir is the per-user root
// (e.g. /data/users); sharedPath is the legacy single-file corpus used when
// UserID == "". Either may be empty.
func NewDiskUserStore(baseDir, sharedPath string) *DiskUserStore {
	return &DiskUserStore{
		baseDir:    baseDir,
		sharedPath: sharedPath,
		cache:      newLRU(50),
		locks:      map[string]*sync.Mutex{},
	}
}

func (s *DiskUserStore) pathFor(userID string) (string, error) {
	if userID == "" {
		return s.sharedPath, nil
	}
	// Defense in depth — the http middleware already validates the id.
	if userID == "." || userID == ".." {
		return "", fmt.Errorf("invalid user id: %q", userID)
	}
	for _, c := range userID {
		if c == '/' || c == '\\' || c == 0 {
			return "", fmt.Errorf("invalid user id: %q", userID)
		}
	}
	if s.baseDir == "" {
		return "", errors.New("no baseDir configured for user store")
	}
	return filepath.Join(s.baseDir, userID, "storyplotter.json"), nil
}

func (s *DiskUserStore) lockFor(userID string) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	if m, ok := s.locks[userID]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.locks[userID] = m
	return m
}

// Load returns the user's parsed Export, or an empty Export if the file
// doesn't exist. Missing files are not an error (first-use fallthrough).
func (s *DiskUserStore) Load(userID string) (*Export, error) {
	s.cacheMu.Lock()
	if e, ok := s.cache.get(userID); ok {
		s.cacheMu.Unlock()
		return e, nil
	}
	s.cacheMu.Unlock()

	path, err := s.pathFor(userID)
	if err != nil {
		return nil, err
	}
	if path == "" {
		// No shared path configured and userID is "".
		empty := &Export{}
		s.cacheMu.Lock()
		s.cache.put(userID, empty)
		s.cacheMu.Unlock()
		return empty, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			empty := &Export{}
			s.cacheMu.Lock()
			s.cache.put(userID, empty)
			s.cacheMu.Unlock()
			return empty, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	exp, err := Parse(b)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	s.cache.put(userID, exp)
	s.cacheMu.Unlock()
	return exp, nil
}

// Save serializes the Export and writes it atomically to the user's path.
// userID == "" is rejected (the shared corpus is read-only from tools).
func (s *DiskUserStore) Save(userID string, exp *Export) error {
	if userID == "" {
		return ErrSharedReadOnly
	}
	lock := s.lockFor(userID)
	lock.Lock()
	defer lock.Unlock()

	path, err := s.pathFor(userID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := Save(path, exp); err != nil {
		return err
	}
	s.cacheMu.Lock()
	s.cache.remove(userID)
	s.cacheMu.Unlock()
	return nil
}

// Raw returns the bytes of the user's JSON file (used for export).
func (s *DiskUserStore) Raw(userID string) ([]byte, error) {
	path, err := s.pathFor(userID)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(path)
}

// Replace validates raw as a StoryPlotter envelope and then writes it
// atomically (preserving the caller's exact formatting). userID == ""
// is rejected.
func (s *DiskUserStore) Replace(userID string, raw []byte) error {
	if userID == "" {
		return ErrSharedReadOnly
	}
	if _, err := Parse(raw); err != nil {
		return fmt.Errorf("invalid StoryPlotter export: %w", err)
	}
	lock := s.lockFor(userID)
	lock.Lock()
	defer lock.Unlock()

	path, err := s.pathFor(userID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := WriteAtomic(path, raw, 0o600); err != nil {
		return err
	}
	s.cacheMu.Lock()
	s.cache.remove(userID)
	s.cacheMu.Unlock()
	return nil
}

// --- tiny LRU cache ----------------------------------------------------

type lruCache struct {
	cap   int
	ll    *list.List
	items map[string]*list.Element
}

type lruEntry struct {
	key string
	val *Export
}

func newLRU(cap int) *lruCache {
	return &lruCache{cap: cap, ll: list.New(), items: map[string]*list.Element{}}
}

func (c *lruCache) get(k string) (*Export, bool) {
	if e, ok := c.items[k]; ok {
		c.ll.MoveToFront(e)
		return e.Value.(*lruEntry).val, true
	}
	return nil, false
}

func (c *lruCache) put(k string, v *Export) {
	if e, ok := c.items[k]; ok {
		c.ll.MoveToFront(e)
		e.Value.(*lruEntry).val = v
		return
	}
	e := c.ll.PushFront(&lruEntry{key: k, val: v})
	c.items[k] = e
	if c.ll.Len() > c.cap {
		back := c.ll.Back()
		if back != nil {
			c.ll.Remove(back)
			delete(c.items, back.Value.(*lruEntry).key)
		}
	}
}

func (c *lruCache) remove(k string) {
	if e, ok := c.items[k]; ok {
		c.ll.Remove(e)
		delete(c.items, k)
	}
}
