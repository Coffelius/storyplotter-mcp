package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type listCharactersArgs struct {
	Plot     string `json:"plot"`
	Priority string `json:"priority"`
}

// ListCharacters returns the list_characters tool.
func ListCharacters() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "list_characters",
			Description: "List characters across all plots, or filtered by plot title and/or priority.",
			InputSchema: schema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plot":     map[string]any{"type": "string"},
					"priority": map[string]any{"type": "string", "description": "e.g. main, supporting."},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a listCharactersArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			var plots []*data.Plot
			if a.Plot != "" {
				p, err := requirePlot(d, a.Plot)
				if err != nil {
					return mcp.ErrorResult(err.Error()), nil
				}
				plots = []*data.Plot{p}
			} else {
				for i := range d.PlotList {
					plots = append(plots, &d.PlotList[i])
				}
			}
			var sb strings.Builder
			total := 0
			for _, p := range plots {
				for _, c := range p.CharList {
					if a.Priority != "" && !strings.EqualFold(c.Priority, a.Priority) {
						continue
					}
					total++
					fmt.Fprintf(&sb, "- [%s] %s  (plot: %s, priority: %s)\n",
						shortField(c, "age"), c.Name(), p.Title, c.Priority)
				}
			}
			if total == 0 {
				return mcp.TextResult("No characters matched."), nil
			}
			return mcp.TextResult(fmt.Sprintf("Found %d character(s):\n%s", total, sb.String())), nil
		},
	}
}

func shortField(c data.Character, key string) string {
	v := c.Field(key)
	if v == "" {
		return "-"
	}
	return v
}
