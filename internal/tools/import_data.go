package tools

import (
	"encoding/json"
	"fmt"

	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type importDataArgs struct {
	Content   string `json:"content"`
	Overwrite *bool  `json:"overwrite,omitempty"`
}

// ImportData returns the import_data tool. It replaces the caller's per-user
// StoryPlotter corpus with the provided JSON payload, after validating that it
// parses as a StoryPlotter export. Requires X-LibreChat-User-Id; the shared
// corpus (empty user id) is read-only.
func ImportData() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "import_data",
			Description: "Import (replace) the caller's StoryPlotter corpus from a JSON export string. Requires user identity (X-LibreChat-User-Id).",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"content"},
				"properties": map[string]any{
					"content":   map[string]any{"type": "string", "description": "Full StoryPlotter export JSON (envelope with memoList/tagColorMap/plotList/allFolderList)."},
					"overwrite": map[string]any{"type": "boolean", "default": true, "description": "If false and data already exists for this user, refuse to replace."},
				},
			}),
		},
		Handler: func(raw json.RawMessage, cc *mcp.CallContext) (mcp.CallToolResult, error) {
			if cc.UserID == "" {
				return mcp.ErrorResult("import_data requires a user identity — set X-LibreChat-User-Id. The shared corpus is read-only."), nil
			}
			var a importDataArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			if a.Content == "" {
				return mcp.ErrorResult("content is required"), nil
			}
			overwrite := true
			if a.Overwrite != nil {
				overwrite = *a.Overwrite
			}
			if !overwrite {
				existing, err := cc.Store.Raw(cc.UserID)
				if err == nil && len(existing) > 0 {
					return mcp.ErrorResult("user already has data; pass overwrite: true to replace"), nil
				}
			}
			if err := cc.Store.Replace(cc.UserID, []byte(a.Content)); err != nil {
				return mcp.ErrorResult("import failed: " + err.Error()), nil
			}
			exp, err := cc.Store.Load(cc.UserID)
			if err != nil {
				return mcp.ErrorResult("imported but failed to re-read: " + err.Error()), nil
			}
			plots := len(exp.PlotList)
			chars := 0
			eras := 0
			for _, p := range exp.PlotList {
				chars += len(p.CharList)
				eras += len(p.EraList)
			}
			folders := len(exp.AllFolderList)
			return mcp.TextResult(fmt.Sprintf("Imported: %d plots, %d characters across all plots, %d eras, %d folders.", plots, chars, eras, folders)), nil
		},
	}
}
