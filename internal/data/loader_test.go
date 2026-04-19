package data

import (
	"path/filepath"
	"testing"
)

func samplePath(t *testing.T) string {
	t.Helper()
	// tests run from the package dir; go back to repo root.
	p, err := filepath.Abs("../../testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadSample(t *testing.T) {
	exp, err := Load(samplePath(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(exp.PlotList), 1; got != want {
		t.Fatalf("PlotList len = %d, want %d", got, want)
	}
	p := exp.PlotList[0]
	if p.Title != "The Crimson Hour" {
		t.Errorf("title = %q", p.Title)
	}
	if got, want := len(p.CharList), 2; got != want {
		t.Errorf("charList len = %d, want %d", got, want)
	}
	if got := p.CharList[0].Name(); got != "Ila Varn" {
		t.Errorf("char name = %q", got)
	}
	if got, want := len(p.SequenceUnitList), 1; got != want {
		t.Errorf("sequenceUnits = %d, want %d", got, want)
	}
	if got, want := len(p.RelationShipList), 1; got != want {
		t.Errorf("relationships = %d, want %d", got, want)
	}
	if got, want := len(p.EraList), 1; got != want {
		t.Errorf("eras = %d, want %d", got, want)
	}
	if got, want := len(p.EraEventList), 1; got != want {
		t.Errorf("eraEvents = %d, want %d", got, want)
	}
	if got, want := len(p.AreaList), 1; got != want {
		t.Errorf("areas = %d, want %d", got, want)
	}
	tags := p.Tags()
	if len(tags) != 2 || tags[0] != "mystery" {
		t.Errorf("tags = %v", tags)
	}
	if got, want := len(exp.AllFolderList), 1; got != want {
		t.Errorf("folders = %d, want %d", got, want)
	}
}

func TestFindPlot(t *testing.T) {
	exp, err := Load(samplePath(t))
	if err != nil {
		t.Fatal(err)
	}
	if p := exp.FindPlot("The Crimson Hour"); p == nil {
		t.Error("exact match failed")
	}
	if p := exp.FindPlot("crimson"); p == nil {
		t.Error("case-insensitive substring failed")
	}
	if p := exp.FindPlot("nope"); p != nil {
		t.Error("unexpected match")
	}
}
