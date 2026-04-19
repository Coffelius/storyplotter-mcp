package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type searchArgs struct {
	Query string `json:"query"`
	Scope string `json:"scope"`
}

// Search returns the search tool.
func Search() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "search",
			Description: "Free-text search across plots, characters, sequence cards, relationships, eras, events, and areas. Scope: plot | character | all.",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"scope": map[string]any{"type": "string", "enum": []string{"plot", "character", "all"}},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a searchArgs
			if err := decodeArgs(raw, &a); err != nil {
				return mcp.ErrorResult("invalid arguments: " + err.Error()), nil
			}
			if a.Query == "" {
				return mcp.ErrorResult("query is required"), nil
			}
			if a.Scope == "" {
				a.Scope = "all"
			}
			q := strings.ToLower(a.Query)
			var hits []string

			for _, p := range d.PlotList {
				if a.Scope == "plot" || a.Scope == "all" {
					if containsFold(p.Title, q) || containsFold(p.Subtitle, q) {
						hits = append(hits, fmt.Sprintf("plot: %s — %s", p.Title, p.Subtitle))
					}
					for _, u := range p.SequenceUnitList {
						if containsFold(u.Title, q) || containsFold(u.Message, q) {
							hits = append(hits, fmt.Sprintf("sequence in %s: %s", p.Title, u.Title))
						}
						for _, c := range u.SequenceCardList {
							if containsFold(c.Idea, q) || containsFold(c.Description, q) || containsFold(c.Memo, q) || containsFold(c.Place, q) {
								hits = append(hits, fmt.Sprintf("card in %s > %s: %s", p.Title, u.Title, truncate(c.Idea+" — "+c.Description, 120)))
							}
						}
					}
					for _, e := range p.EraList {
						if containsFold(e.Title, q) || containsFold(e.Description, q) {
							hits = append(hits, fmt.Sprintf("era in %s: %s", p.Title, e.Title))
						}
					}
					for _, ev := range p.EraEventList {
						if containsFold(ev.Title, q) || containsFold(ev.Description, q) {
							hits = append(hits, fmt.Sprintf("event in %s: %s", p.Title, ev.Title))
						}
					}
					for _, ar := range p.AreaList {
						if containsFold(ar.Title, q) || containsFold(ar.Description, q) {
							hits = append(hits, fmt.Sprintf("area in %s: %s", p.Title, ar.Title))
						}
					}
					for _, r := range p.RelationShipList {
						if containsFold(r.Label, q) || containsFold(r.Description, q) {
							hits = append(hits, fmt.Sprintf("relationship in %s: %s", p.Title, r.Label))
						}
					}
				}
				if a.Scope == "character" || a.Scope == "all" {
					for _, c := range p.CharList {
						if characterMatches(c, q) {
							hits = append(hits, fmt.Sprintf("character in %s: %s", p.Title, c.Name()))
						}
					}
				}
			}
			if len(hits) == 0 {
				return mcp.TextResult(fmt.Sprintf("No results for %q (scope=%s).", a.Query, a.Scope)), nil
			}
			return mcp.TextResult(fmt.Sprintf("Found %d result(s) for %q:\n- %s",
				len(hits), a.Query, strings.Join(hits, "\n- "))), nil
		},
	}
}

func characterMatches(c data.Character, q string) bool {
	if containsFold(c.Name(), q) {
		return true
	}
	for _, f := range c.CharParam {
		if containsFold(f.Value, q) || containsFold(f.Name, q) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
