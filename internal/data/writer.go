package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path atomically: creates a temp file in the
// same directory (critical — cross-device rename would not be atomic),
// fsyncs, closes, then os.Renames over the destination. The resulting file
// has the requested perm; if perm is 0 we default to 0600.
func WriteAtomic(path string, blob []byte, perm os.FileMode) error {
	if perm == 0 {
		perm = 0o600
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".storyplotter-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := f.Name()
	// Best-effort cleanup if we bail early.
	defer func() {
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := f.Write(blob); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Save serializes an Export back to the StoryPlotter envelope (nested JSON
// fields stored as strings) and writes it atomically to path.
func Save(path string, exp *Export) error {
	blob, err := Marshal(exp)
	if err != nil {
		return err
	}
	return WriteAtomic(path, blob, 0o600)
}

// Marshal produces the StoryPlotter envelope bytes for an Export. Inverse
// of Parse: each of the 4 nested collections is JSON-marshalled to a
// string, then wrapped in RawExport and marshalled again.
func Marshal(exp *Export) ([]byte, error) {
	if exp == nil {
		exp = &Export{}
	}
	memoList, err := marshalStringField(exp.MemoList, "[]")
	if err != nil {
		return nil, fmt.Errorf("marshal memoList: %w", err)
	}
	tagColorMap, err := marshalStringField(exp.TagColorMap, "{}")
	if err != nil {
		return nil, fmt.Errorf("marshal tagColorMap: %w", err)
	}
	plotList, err := marshalStringField(exp.PlotList, "[]")
	if err != nil {
		return nil, fmt.Errorf("marshal plotList: %w", err)
	}
	allFolderList, err := marshalStringField(exp.AllFolderList, "[]")
	if err != nil {
		return nil, fmt.Errorf("marshal allFolderList: %w", err)
	}
	raw := RawExport{
		MemoList:      memoList,
		TagColorMap:   tagColorMap,
		PlotList:      plotList,
		AllFolderList: allFolderList,
	}
	return json.Marshal(raw)
}

func marshalStringField(v any, zero string) (string, error) {
	if v == nil {
		return zero, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	// An empty slice/map marshals to "[]"/"{}" which is fine; nil slices
	// marshal to "null" though, so normalise to the zero value.
	if string(b) == "null" {
		return zero, nil
	}
	return string(b), nil
}
