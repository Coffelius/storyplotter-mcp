package tools

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabistuff/storyplotter-mcp/internal/data"
)

func loadFixture(t *testing.T) *data.Export {
	t.Helper()
	p, err := filepath.Abs("../../testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	exp, err := data.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return exp
}

func runTool(t *testing.T, tool Tool, args map[string]any) string {
	t.Helper()
	var raw json.RawMessage
	if args != nil {
		b, _ := json.Marshal(args)
		raw = b
	}
	res, err := tool.Handler(raw, loadFixture(t))
	if err != nil {
		t.Fatalf("%s: err %v", tool.Def.Name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("%s: empty content", tool.Def.Name)
	}
	return res.Content[0].Text
}

func TestAllToolsReturnDefinitions(t *testing.T) {
	got := All()
	// 8 tools required by the issue; we expose 9 (list_eras and list_events
	// count as two entries per the issue description).
	if len(got) != 9 {
		t.Errorf("want 9 tools, got %d", len(got))
	}
	for _, tool := range got {
		if tool.Def.Name == "" {
			t.Errorf("tool missing name")
		}
		if len(tool.Def.InputSchema) == 0 {
			t.Errorf("tool %s missing inputSchema", tool.Def.Name)
		}
	}
}

func TestListPlots(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"all", nil, "The Crimson Hour"},
		{"by_status_writing", map[string]any{"status": "writing"}, "The Crimson Hour"},
		{"by_folder", map[string]any{"folder": "fantasy"}, "The Crimson Hour"},
		{"by_tag", map[string]any{"tag": "mystery"}, "The Crimson Hour"},
		{"by_status_written_no_match", map[string]any{"status": "written"}, "No plots matched"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runTool(t, ListPlots(), tc.args)
			if !strings.Contains(out, tc.want) {
				t.Errorf("output = %q; want contains %q", out, tc.want)
			}
		})
	}
}

func TestGetPlot(t *testing.T) {
	out := runTool(t, GetPlot(), map[string]any{"title": "Crimson"})
	if !strings.Contains(out, "The Crimson Hour") {
		t.Errorf("out = %s", out)
	}
	out = runTool(t, GetPlot(), map[string]any{"title": "nope"})
	if !strings.Contains(out, "plot not found") {
		t.Errorf("expected not found, got %s", out)
	}
}

func TestGetPlot_FolderFallback(t *testing.T) {
	// Three plots across two folders: one unique in "sci-fi",
	// two sharing "fantasy" to force disambiguation.
	exp := &data.Export{PlotList: []data.Plot{
		{Title: "The Crimson Hour", FolderPath: "fantasy"},
		{Title: "Second Dawn", FolderPath: "fantasy"},
		{Title: "The Tin Star", FolderPath: "sci-fi"},
	}}

	call := func(title string) string {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"title": title})
		res, err := GetPlot().Handler(raw, exp)
		if err != nil {
			t.Fatal(err)
		}
		return res.Content[0].Text
	}

	if out := call("sci-fi"); !strings.Contains(out, "The Tin Star") {
		t.Errorf("folder unique: %s", out)
	}
	out := call("fantasy")
	if !strings.Contains(out, "2 plots matched folder") ||
		!strings.Contains(out, "The Crimson Hour") ||
		!strings.Contains(out, "Second Dawn") {
		t.Errorf("folder disambiguation: %s", out)
	}
	// Title match still wins over folder-only candidates.
	if out := call("Crimson"); !strings.Contains(out, "The Crimson Hour") ||
		strings.Contains(out, "plots matched folder") {
		t.Errorf("title precedence: %s", out)
	}
	// Unmatched query still errors cleanly.
	if out := call("nothing"); !strings.Contains(out, "plot not found") {
		t.Errorf("missing: %s", out)
	}
}

func TestListCharacters(t *testing.T) {
	out := runTool(t, ListCharacters(), nil)
	if !strings.Contains(out, "Ila Varn") || !strings.Contains(out, "Koren Bly") {
		t.Errorf("missing chars: %s", out)
	}
	out = runTool(t, ListCharacters(), map[string]any{"priority": "main"})
	if !strings.Contains(out, "Ila Varn") || strings.Contains(out, "Koren Bly") {
		t.Errorf("priority filter failed: %s", out)
	}
}

func TestGetCharacter(t *testing.T) {
	out := runTool(t, GetCharacter(), map[string]any{"plot": "The Crimson Hour", "name": "Ila"})
	if !strings.Contains(out, "Ila Varn") || !strings.Contains(out, "scribe") {
		t.Errorf("profile wrong: %s", out)
	}
}

func TestListRelationships(t *testing.T) {
	out := runTool(t, ListRelationships(), map[string]any{"plot": "The Crimson Hour"})
	if !strings.Contains(out, "meets") || !strings.Contains(out, "Ila Varn") {
		t.Errorf("relationships wrong: %s", out)
	}
	out = runTool(t, ListRelationships(), map[string]any{"plot": "The Crimson Hour", "character": "Koren"})
	if !strings.Contains(out, "Koren Bly") {
		t.Errorf("character filter failed: %s", out)
	}
}

func TestListEras(t *testing.T) {
	out := runTool(t, ListEras(), map[string]any{"plot": "The Crimson Hour"})
	if !strings.Contains(out, "The Long Dusk") {
		t.Errorf("eras: %s", out)
	}
}

func TestListEvents(t *testing.T) {
	out := runTool(t, ListEvents(), map[string]any{"plot": "The Crimson Hour"})
	if !strings.Contains(out, "The Stalling") {
		t.Errorf("events: %s", out)
	}
	out = runTool(t, ListEvents(), map[string]any{"plot": "The Crimson Hour", "era": "The Long Dusk"})
	if !strings.Contains(out, "The Stalling") {
		t.Errorf("filtered events: %s", out)
	}
}

func TestSearch(t *testing.T) {
	cases := []struct {
		name, q, scope, want string
	}{
		{"plot_title", "Crimson", "plot", "The Crimson Hour"},
		{"character", "scribe", "character", "Ila Varn"},
		{"all_area", "Marrow", "all", "Marrow Keep"},
		{"none", "zzzzz", "all", "No results"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runTool(t, Search(), map[string]any{"query": tc.q, "scope": tc.scope})
			if !strings.Contains(out, tc.want) {
				t.Errorf("got %s; want %q", out, tc.want)
			}
		})
	}
}

func TestGenerateContext(t *testing.T) {
	out := runTool(t, GenerateContext(), map[string]any{
		"plot":  "The Crimson Hour",
		"focus": "Write the meeting scene.",
	})
	for _, want := range []string{"# System", "# Focus", "The Crimson Hour", "Ila Varn"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %s", want, out)
		}
	}

	// truncation
	out = runTool(t, GenerateContext(), map[string]any{
		"plot":      "The Crimson Hour",
		"focus":     "x",
		"maxTokens": 20,
	})
	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation, got: %s", out)
	}
}
