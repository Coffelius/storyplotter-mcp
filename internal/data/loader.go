package data

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads a StoryPlotter export file and parses the nested JSON.
func Load(path string) (*Export, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(b)
}

// Parse parses StoryPlotter export bytes.
func Parse(b []byte) (*Export, error) {
	var raw RawExport
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	out := &Export{
		TagColorMap: map[string]json.RawMessage{},
	}
	if s := raw.MemoList; s != "" {
		if err := json.Unmarshal([]byte(s), &out.MemoList); err != nil {
			return nil, fmt.Errorf("unmarshal memoList: %w", err)
		}
	}
	if s := raw.TagColorMap; s != "" {
		if err := json.Unmarshal([]byte(s), &out.TagColorMap); err != nil {
			return nil, fmt.Errorf("unmarshal tagColorMap: %w", err)
		}
	}
	if s := raw.PlotList; s != "" {
		if err := json.Unmarshal([]byte(s), &out.PlotList); err != nil {
			return nil, fmt.Errorf("unmarshal plotList: %w", err)
		}
	}
	if s := raw.AllFolderList; s != "" {
		if err := json.Unmarshal([]byte(s), &out.AllFolderList); err != nil {
			return nil, fmt.Errorf("unmarshal allFolderList: %w", err)
		}
	}
	return out, nil
}

// FindPlot returns the first plot matching by exact title, then case-insensitive substring.
func (e *Export) FindPlot(title string) *Plot {
	for i := range e.PlotList {
		if e.PlotList[i].Title == title {
			return &e.PlotList[i]
		}
	}
	lower := toLower(title)
	for i := range e.PlotList {
		if contains(toLower(e.PlotList[i].Title), lower) {
			return &e.PlotList[i]
		}
	}
	return nil
}

// FindPlotsByFolder returns every plot whose FolderPath matches the query
// (exact match first; if none, case-insensitive substring). Used as a
// fallback when a title search fails — plot titles in real exports are
// often auto-generated timestamps and users refer to plots by folder.
func (e *Export) FindPlotsByFolder(folder string) []*Plot {
	if folder == "" {
		return nil
	}
	var exact []*Plot
	for i := range e.PlotList {
		if e.PlotList[i].FolderPath == folder {
			exact = append(exact, &e.PlotList[i])
		}
	}
	if len(exact) > 0 {
		return exact
	}
	lower := toLower(folder)
	var fuzzy []*Plot
	for i := range e.PlotList {
		if contains(toLower(e.PlotList[i].FolderPath), lower) {
			fuzzy = append(fuzzy, &e.PlotList[i])
		}
	}
	return fuzzy
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
