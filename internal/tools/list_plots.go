package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type listPlotsArgs struct {
	Folder string `json:"folder"`
	Status string `json:"status"`
	Tag    string `json:"tag"`
}

// ListPlots returns the list_plots tool.
func ListPlots() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "list_plots",
			Description: "List plots, optionally filtered by folder, writing status, or tag.",
			InputSchema: schema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"folder": map[string]any{"type": "string", "description": "Folder path filter (exact or prefix)."},
					"status": map[string]any{"type": "string", "enum": []string{"unwritten", "writing", "written"}},
					"tag":    map[string]any{"type": "string"},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a listPlotsArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			var out []map[string]any
			for _, p := range d.PlotList {
				if a.Folder != "" && p.FolderPath != a.Folder && !strings.HasPrefix(p.FolderPath, a.Folder+"/") {
					continue
				}
				if a.Status != "" && p.WritingStatus != a.Status {
					continue
				}
				if a.Tag != "" {
					found := false
					for _, t := range p.Tags() {
						if strings.EqualFold(t, a.Tag) {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
				out = append(out, map[string]any{
					"title":      p.Title,
					"subtitle":   p.Subtitle,
					"status":     p.WritingStatus,
					"folderPath": p.FolderPath,
					"tags":       p.Tags(),
					"characters": len(p.CharList),
				})
			}
			text := formatPlotsText(out)
			b, _ := json.MarshalIndent(out, "", "  ")
			return mcp.TextResult(text + "\n\n" + string(b)), nil
		},
	}
}

func formatPlotsText(plots []map[string]any) string {
	if len(plots) == 0 {
		return "No plots matched."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d plot(s):\n", len(plots))
	for _, p := range plots {
		fmt.Fprintf(&sb, "- %s [%s] (%s)\n", p["title"], p["status"], p["folderPath"])
	}
	return sb.String()
}
