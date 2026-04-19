package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type generateContextArgs struct {
	Plot             string   `json:"plot"`
	Focus            string   `json:"focus"`
	TargetCharacters []string `json:"targetCharacters"`
	MaxTokens        int      `json:"maxTokens"`
}

// GenerateContext returns the generate_context tool.
func GenerateContext() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name: "generate_context",
			Description: "Build a structured writing-assistant context for a plot: system prompt, " +
				"relevant facts, and character profiles. maxTokens is approximated as chars/4.",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"plot", "focus"},
				"properties": map[string]any{
					"plot":             map[string]any{"type": "string"},
					"focus":            map[string]any{"type": "string", "description": "What the author wants help with."},
					"targetCharacters": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"maxTokens":        map[string]any{"type": "integer", "description": "Soft cap (default 4000)."},
				},
			}),
		},
		Handler: func(raw json.RawMessage, cc *mcp.CallContext) (mcp.CallToolResult, error) {
			d, err := cc.Store.Load(cc.UserID)
			if err != nil {
				return mcp.ErrorResult("load data: " + err.Error()), nil
			}
			var a generateContextArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			if a.Plot == "" || a.Focus == "" {
				return mcp.ErrorResult("plot and focus are required"), nil
			}
			if a.MaxTokens <= 0 {
				a.MaxTokens = 4000
			}
			maxChars := a.MaxTokens * 4
			p, err := requirePlot(d, a.Plot)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}

			var sb strings.Builder
			sb.WriteString("# System\nYou are a creative writing collaborator working inside the StoryPlotter universe. ")
			sb.WriteString("Stay consistent with the established world, characters, and timeline. Ask concise questions when anything is ambiguous.\n\n")
			fmt.Fprintf(&sb, "# Focus\n%s\n\n", a.Focus)
			fmt.Fprintf(&sb, "# Plot: %s\n", p.Title)
			if p.Subtitle != "" {
				fmt.Fprintf(&sb, "_%s_\n", p.Subtitle)
			}
			fmt.Fprintf(&sb, "Status: %s  |  Type: %s  |  Tags: %s\n\n",
				p.WritingStatus, p.PlotType, strings.Join(p.Tags(), ", "))

			// Targeted character profiles.
			chars := a.TargetCharacters
			if len(chars) == 0 {
				// default: include main-priority characters.
				for _, c := range p.CharList {
					if strings.EqualFold(c.Priority, "main") {
						chars = append(chars, c.Name())
					}
				}
				// fall back to all characters if no "main" priority
				if len(chars) == 0 {
					for _, c := range p.CharList {
						chars = append(chars, c.Name())
					}
				}
			}
			if len(chars) > 0 {
				sb.WriteString("# Character Profiles\n")
				for _, name := range chars {
					c := findCharacter(p, name)
					if c == nil {
						continue
					}
					sb.WriteString(renderCharacter(p, c))
					sb.WriteString("\n")
				}
			}

			// Relevant facts: eras + areas.
			if len(p.EraList) > 0 {
				sb.WriteString("# Timeline\n")
				for _, e := range p.EraList {
					fmt.Fprintf(&sb, "- %s (%s - %s): %s\n", e.Title, e.StartTime, e.EndTime, e.Description)
				}
				sb.WriteString("\n")
			}
			if len(p.AreaList) > 0 {
				sb.WriteString("# Locations\n")
				for _, ar := range p.AreaList {
					fmt.Fprintf(&sb, "- %s [%s]: %s\n", ar.Title, ar.Category, ar.Description)
				}
				sb.WriteString("\n")
			}
			if len(p.RelationShipList) > 0 {
				sb.WriteString("# Relationships\n")
				for _, r := range p.RelationShipList {
					fmt.Fprintf(&sb, "- %s --[%s]--> %s: %s\n",
						charNameAt(p, r.FromIndex), r.Label, charNameAt(p, r.ToIndex), r.Description)
				}
				sb.WriteString("\n")
			}

			out := sb.String()
			if len(out) > maxChars {
				out = out[:maxChars] + "\n\n[truncated: exceeded ~" + fmt.Sprint(a.MaxTokens) + " tokens]"
			}
			return mcp.TextResult(out), nil
		},
	}
}

// silence unused
var _ = data.Export{}
