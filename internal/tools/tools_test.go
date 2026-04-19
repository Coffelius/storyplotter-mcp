package tools

import (
	"context"
	"encoding/json"
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

// runToolAsUser runs a tool with a specific UserID and optional signer /
// publicURL against the provided store.
func runToolAsUser(t *testing.T, store mcp.UserStore, signer *mcp.TokenSigner, publicURL string, tool Tool, args map[string]any, userID string) (string, bool) {
	t.Helper()
	var raw json.RawMessage
	if args != nil {
		b, _ := json.Marshal(args)
		raw = b
	}
	cc := &mcp.CallContext{
		Ctx:       context.Background(),
		UserID:    userID,
		Store:     store,
		Signer:    signer,
		PublicURL: publicURL,
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

func TestRequestExportLink_HappyPath(t *testing.T) {
	dir := t.TempDir()
	store := data.NewDiskUserStore(dir, "")
	// Seed alice's corpus with a minimal valid envelope.
	body := []byte(`{"memoList":"[]","tagColorMap":"{}","plotList":"[]","allFolderList":"[]"}`)
	if err := store.Replace("alice", body); err != nil {
		t.Fatal(err)
	}
	signer := mcp.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"))

	out, isErr := runToolAsUser(t, store, signer, "https://example.test", RequestExportLink(), nil, "alice")
	if isErr {
		t.Fatalf("tool errored: %s", out)
	}
	if !strings.Contains(out, "/download?t=") {
		t.Errorf("expected /download?t= in output, got: %s", out)
	}
	if !strings.Contains(out, "https://example.test") {
		t.Errorf("expected configured publicURL in output, got: %s", out)
	}
}

func TestRequestExportLink_RequiresUserID(t *testing.T) {
	store := data.NewDiskUserStore(t.TempDir(), "")
	signer := mcp.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"))
	out, isErr := runToolAsUser(t, store, signer, "", RequestExportLink(), nil, "")
	if !isErr {
		t.Fatalf("expected error result, got: %s", out)
	}
	if !strings.Contains(out, "requires a user identity") {
		t.Errorf("unexpected error text: %s", out)
	}
}

func TestRequestExportLink_NoDataYet(t *testing.T) {
	store := data.NewDiskUserStore(t.TempDir(), "")
	signer := mcp.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"))
	out, isErr := runToolAsUser(t, store, signer, "", RequestExportLink(), nil, "bob")
	if !isErr {
		t.Fatalf("expected error result, got: %s", out)
	}
	if !strings.Contains(out, "no data imported yet") {
		t.Errorf("unexpected error text: %s", out)
	}
}

func TestAllToolsReturnDefinitions(t *testing.T) {
	got := All()
	// 9 read-only tools (list_eras and list_events count as two entries
	// per the original issue), plus request_export_link (GAB-95) brings
	// it to 10. import_data lands separately (GAB-94) and will push this
	// to 11 at central merge.
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
