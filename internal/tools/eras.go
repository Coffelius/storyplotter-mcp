package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

type listErasArgs struct {
	Plot string `json:"plot"`
}

// ListEras returns the list_eras tool.
func ListEras() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "list_eras",
			Description: "List eras defined on a plot's timeline.",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"plot"},
				"properties": map[string]any{
					"plot": map[string]any{"type": "string"},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a listErasArgs
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
			if len(p.EraList) == 0 {
				return mcp.TextResult("No eras defined."), nil
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "Found %d era(s) in %s:\n", len(p.EraList), p.Title)
			for _, e := range p.EraList {
				fmt.Fprintf(&sb, "- %s (%s - %s)", e.Title, e.StartTime, e.EndTime)
				if e.Description != "" {
					fmt.Fprintf(&sb, " : %s", e.Description)
				}
				sb.WriteString("\n")
			}
			return mcp.TextResult(sb.String()), nil
		},
	}
}

type listEventsArgs struct {
	Plot string `json:"plot"`
	Era  string `json:"era"`
}

// ListEvents returns the list_events tool.
func ListEvents() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "list_events",
			Description: "List era events on a plot's timeline, optionally filtered to a specific era title.",
			InputSchema: schema(map[string]any{
				"type":     "object",
				"required": []string{"plot"},
				"properties": map[string]any{
					"plot": map[string]any{"type": "string"},
					"era":  map[string]any{"type": "string"},
				},
			}),
		},
		Handler: func(raw json.RawMessage, d *data.Export) (mcp.CallToolResult, error) {
			var a listEventsArgs
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
			eraIdx := -1
			if a.Era != "" {
				for i, e := range p.EraList {
					if strings.EqualFold(e.Title, a.Era) {
						eraIdx = i
						break
					}
				}
				if eraIdx < 0 {
					return mcp.ErrorResult("era not found: " + a.Era), nil
				}
			}
			var sb strings.Builder
			count := 0
			for _, ev := range p.EraEventList {
				if eraIdx >= 0 && ev.EraIndex != eraIdx {
					continue
				}
				count++
				eraName := "-"
				if ev.EraIndex >= 0 && ev.EraIndex < len(p.EraList) {
					eraName = p.EraList[ev.EraIndex].Title
				}
				fmt.Fprintf(&sb, "- [%s] %s (%s - %s)", eraName, ev.Title, ev.StartTime, ev.EndTime)
				if ev.Description != "" {
					fmt.Fprintf(&sb, " : %s", ev.Description)
				}
				sb.WriteString("\n")
			}
			if count == 0 {
				return mcp.TextResult("No events matched."), nil
			}
			return mcp.TextResult(fmt.Sprintf("Found %d event(s):\n%s", count, sb.String())), nil
		},
	}
}

// silence unused
var _ = data.Plot{}
