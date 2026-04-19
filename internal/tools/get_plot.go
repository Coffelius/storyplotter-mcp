package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gabistuff/storyplotter-mcp/internal/data"
	"github.com/gabistuff/storyplotter-mcp/internal/mcp"
)

type getPlotArgs struct {
	Title string `json:"title"`
}

// GetPlot returns the get_plot tool.
func GetPlot() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "get_plot",
			Description: "Fetch a plot by title (exact, then case-insensitive substring). If no title matches, falls back to matching the folder path; when multiple plots share a folder, returns a disambiguation list of candidate titles.",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"title"},
				"properties": map[string]any{
					"title": map[string]any{"type": "string"},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a getPlotArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			if a.Title == "" {
				return mcp.ErrorResult("title is required"), nil
			}
			p := d.FindPlot(a.Title)
			if p == nil {
				candidates := d.FindPlotsByFolder(a.Title)
				switch len(candidates) {
				case 0:
					return mcp.ErrorResult("plot not found: " + a.Title), nil
				case 1:
					p = candidates[0]
				default:
					var sb strings.Builder
					fmt.Fprintf(&sb, "%d plots matched folder %q — pass an exact title:\n",
						len(candidates), a.Title)
					for _, c := range candidates {
						fmt.Fprintf(&sb, "- %s (folder: %s)\n", c.Title, c.FolderPath)
					}
					return mcp.ErrorResult(sb.String()), nil
				}
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "# %s\n", p.Title)
			if p.Subtitle != "" {
				fmt.Fprintf(&sb, "_%s_\n", p.Subtitle)
			}
			fmt.Fprintf(&sb, "\nStatus: %s\nFolder: %s\nType: %s\nTags: %s\n",
				p.WritingStatus, p.FolderPath, p.PlotType, strings.Join(p.Tags(), ", "))
			fmt.Fprintf(&sb, "Characters: %d\nSequence units: %d\nRelationships: %d\nEras: %d\nEra events: %d\nAreas: %d\n",
				len(p.CharList), len(p.SequenceUnitList), len(p.RelationShipList),
				len(p.EraList), len(p.EraEventList), len(p.AreaList))

			if len(p.SequenceUnitList) > 0 {
				sb.WriteString("\n## Sequence\n")
				for _, u := range p.SequenceUnitList {
					fmt.Fprintf(&sb, "- [%s] %s (%d cards)\n", u.Category, u.Title, len(u.SequenceCardList))
				}
			}
			return mcp.TextResult(sb.String()), nil
		},
	}
}

// ensure fmt kept usable even if no refs removed in future edits.
var _ = fmt.Sprintf

// ensure data import referenced (silence if helper moves).
var _ *data.Plot
