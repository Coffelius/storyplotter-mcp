package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type getCharacterArgs struct {
	Plot string `json:"plot"`
	Name string `json:"name"`
}

// GetCharacter returns the get_character tool.
func GetCharacter() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "get_character",
			Description: "Fetch a character's full profile from a plot.",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"plot", "name"},
				"properties": map[string]any{
					"plot": map[string]any{"type": "string"},
					"name": map[string]any{"type": "string"},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a getCharacterArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			if a.Plot == "" || a.Name == "" {
				return mcp.ErrorResult("plot and name are required"), nil
			}
			p, err := requirePlot(d, a.Plot)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			c := findCharacter(p, a.Name)
			if c == nil {
				return mcp.ErrorResult(fmt.Sprintf("character not found: %s", a.Name)), nil
			}
			return mcp.TextResult(renderCharacter(p, c)), nil
		},
	}
}

func renderCharacter(p *data.Plot, c *data.Character) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n", c.Name())
	fmt.Fprintf(&sb, "Plot: %s\nPriority: %s\n", p.Title, c.Priority)
	// ordered fields
	keys := make([]string, 0, len(c.CharParam))
	for k := range c.CharParam {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return c.CharParam[keys[i]].Sort < c.CharParam[keys[j]].Sort
	})
	sb.WriteString("\n## Attributes\n")
	for _, k := range keys {
		f := c.CharParam[k]
		if f.IsSilent || f.Value == "" {
			continue
		}
		label := f.Name
		if label == "" {
			label = k
		}
		fmt.Fprintf(&sb, "- %s: %s\n", label, f.Value)
	}
	return sb.String()
}
