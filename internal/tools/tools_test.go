package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

// fixtureStore returns a DiskUserStore whose shared corpus points at the
// repository's testdata sample, and whose per-user dir is a tmpdir. Using
// the real store exercises Load/Parse end-to-end.
func fixtureStore(t *testing.T) *data.DiskUserStore {
	t.Helper()
	p, err := filepath.Abs("../../testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	return data.NewDiskUserStore(t.TempDir(), p)
}

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

// exportStore wraps a preloaded Export for tests that synthesise fixtures
// inline (no disk round-trip needed).
type exportStore struct{ exp *data.Export }

func (e *exportStore) Load(string) (*data.Export, error) {
	if e.exp == nil {
		return &data.Export{}, nil
	}
	return e.exp, nil
}
func (*exportStore) Save(string, *data.Export) error { return nil }
func (*exportStore) Raw(string) ([]byte, error)      { return nil, nil }
func (*exportStore) Replace(string, []byte) error    { return nil }

func newCallContext(t *testing.T, exp *data.Export) *mcp.CallContext {
	t.Helper()
	return &mcp.CallContext{
		Ctx:    context.Background(),
		UserID: "",
		Store:  &exportStore{exp: exp},
	}
}

func newSharedCallContext(t *testing.T) *mcp.CallContext {
	t.Helper()
	return &mcp.CallContext{
		Ctx:    context.Background(),
		UserID: "",
		Store:  fixtureStore(t),
	}
}

func runTool(t *testing.T, tool Tool, args map[string]any) string {
	t.Helper()
	var raw json.RawMessage
	if args != nil {
		b, _ := json.Marshal(args)
		raw = b
	}
	res, err := tool.Handler(raw, newSharedCallContext(t))
	if err != nil {
		t.Fatalf("%s: err %v", tool.Def.Name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("%s: empty content", tool.Def.Name)
	}
	return res.Content[0].Text
}

// keep loadFixture referenced for inline fixture tests.
var _ = loadFixture

// runToolAsUser runs a tool with a specific UserID against a DiskUserStore
// whose per-user dir is a tmpdir (persistent across calls within a test if
// the same store is reused). Callers that need store reuse should build the
// store once and call the tool handler directly.
func runToolAsUser(t *testing.T, store mcp.UserStore, tool Tool, args map[string]any, userID string) (string, bool) {
	t.Helper()
	var raw json.RawMessage
	if args != nil {
		b, _ := json.Marshal(args)
		raw = b
	}
	cc := &mcp.CallContext{
		Ctx:    context.Background(),
		UserID: userID,
		Store:  store,
	}
	res, err := tool.Handler(raw, cc)
	if err != nil {
		t.Fatalf("%s: err %v", tool.Def.Name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("%s: empty content", tool.Def.Name)
	}
	return res.Content[0].Text, res.IsError
}

func TestImportData_HappyPath(t *testing.T) {
	store := data.NewDiskUserStore(t.TempDir(), "")
	p, err := filepath.Abs("../../testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	out, isErr := runToolAsUser(t, store, ImportData(), map[string]any{
		"content": string(content),
	}, "alice")
	if isErr {
		t.Fatalf("import returned error: %s", out)
	}
	if !strings.Contains(out, "Imported:") || !strings.Contains(out, "plots") {
		t.Errorf("unexpected import output: %s", out)
	}
	// list_plots for alice should now see the sample plots.
	out, isErr = runToolAsUser(t, store, ListPlots(), nil, "alice")
	if isErr {
		t.Fatalf("list_plots errored: %s", out)
	}
	if !strings.Contains(out, "The Crimson Hour") {
		t.Errorf("alice list_plots missing imported plot: %s", out)
	}
	// bob never imported — should be empty.
	out, _ = runToolAsUser(t, store, ListPlots(), nil, "bob")
	if !strings.Contains(out, "No plots") {
		t.Errorf("bob should see no plots, got: %s", out)
	}
}

func TestImportData_RequiresUserID(t *testing.T) {
	store := data.NewDiskUserStore(t.TempDir(), "")
	out, isErr := runToolAsUser(t, store, ImportData(), map[string]any{
		"content": `{"memoList":"[]","tagColorMap":"{}","plotList":"[]","allFolderList":"[]"}`,
	}, "")
	if !isErr {
		t.Fatalf("expected error result for empty user id, got: %s", out)
	}
	if !strings.Contains(out, "requires a user identity") {
		t.Errorf("unexpected error text: %s", out)
	}
}

func TestImportData_InvalidJSON(t *testing.T) {
	store := data.NewDiskUserStore(t.TempDir(), "")
	out, isErr := runToolAsUser(t, store, ImportData(), map[string]any{
		"content": "not valid json at all",
	}, "carol")
	if !isErr {
		t.Fatalf("expected error result, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "import failed") && !strings.Contains(strings.ToLower(out), "invalid") {
		t.Errorf("unexpected error text: %s", out)
	}
	// Verify nothing was written: Raw should be empty / not exist.
	raw, err := store.Raw("carol")
	if err == nil && len(raw) > 0 {
		t.Errorf("expected no bytes written on invalid import, got %d bytes", len(raw))
	}
}

func TestImportData_NoOverwrite(t *testing.T) {
	store := data.NewDiskUserStore(t.TempDir(), "")
	p, err := filepath.Abs("../../testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// First import succeeds.
	_, isErr := runToolAsUser(t, store, ImportData(), map[string]any{
		"content": string(content),
	}, "dave")
	if isErr {
		t.Fatal("first import should succeed")
	}
	// Second import with overwrite:false should fail.
	out, isErr := runToolAsUser(t, store, ImportData(), map[string]any{
		"content":   string(content),
		"overwrite": false,
	}, "dave")
	if !isErr {
		t.Fatalf("expected error when overwrite=false over existing data, got: %s", out)
	}
	if !strings.Contains(out, "overwrite: true") {
		t.Errorf("unexpected error text: %s", out)
	}
}

func TestAllToolsReturnDefinitions(t *testing.T) {
	got := All()
	// 8 tools required by the original issue, we expose 9 (list_eras and
	// list_events count as two entries per that description), plus
	// import_data (GAB-94) brings it to 10.
	if len(got) != 10 {
		t.Errorf("want 10 tools, got %d", len(got))
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
		res, err := GetPlot().Handler(raw, newCallContext(t, exp))
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
