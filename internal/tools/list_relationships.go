package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type listRelArgs struct {
	Plot      string `json:"plot"`
	Character string `json:"character"`
}

// ListRelationships returns the list_relationships tool.
func ListRelationships() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "list_relationships",
			Description: "List relationships in a plot, optionally filtered to those involving a named character.",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"plot"},
				"properties": map[string]any{
					"plot":      map[string]any{"type": "string"},
					"character": map[string]any{"type": "string"},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a listRelArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			if a.Plot == "" {
				return mcp.ErrorResult("plot is required"), nil
			}
			p, err := requirePlot(d, a.Plot)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			filterIdx := -1
			if a.Character != "" {
				c := findCharacter(p, a.Character)
				if c == nil {
					return mcp.ErrorResult("character not found: " + a.Character), nil
				}
				for i := range p.CharList {
					if &p.CharList[i] == c {
						filterIdx = i
						break
					}
				}
			}
			var sb strings.Builder
			count := 0
			for _, r := range p.RelationShipList {
				if filterIdx >= 0 && r.FromIndex != filterIdx && r.ToIndex != filterIdx {
					continue
				}
				count++
				from := charNameAt(p, r.FromIndex)
				to := charNameAt(p, r.ToIndex)
				fmt.Fprintf(&sb, "- %s --[%s]--> %s", from, r.Label, to)
				if r.Description != "" {
					fmt.Fprintf(&sb, " : %s", r.Description)
				}
				sb.WriteString("\n")
			}
			if count == 0 {
				return mcp.TextResult("No relationships matched."), nil
			}
			return mcp.TextResult(fmt.Sprintf("Found %d relationship(s):\n%s", count, sb.String())), nil
		},
	}
}

func charNameAt(p *data.Plot, i int) string {
	if i < 0 || i >= len(p.CharList) {
		return fmt.Sprintf("<index %d>", i)
	}
	return p.CharList[i].Name()
}
