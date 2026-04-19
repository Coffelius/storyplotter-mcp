// Package tools contains the MCP tool implementations.
package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

// Tool is a concrete mcp.Tool registration.
type Tool = mcp.Tool

// All returns every tool this package exposes.
func All() []mcp.Tool {
	return []mcp.Tool{
		ListPlots(),
		GetPlot(),
		ListCharacters(),
		GetCharacter(),
		ListRelationships(),
		ListEras(),
		ListEvents(),
		Search(),
		GenerateContext(),
	}
}

// schema is a helper to build an inputSchema raw message.
func schema(s any) json.RawMessage {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return b
}

// decodeArgs unmarshals optional arguments.
func decodeArgs(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, v)
}

func lower(s string) string { return strings.ToLower(s) }

func containsFold(hay, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(hay), strings.ToLower(needle))
}

// findCharacter returns a pointer to a character in the plot by name
// (exact first, then case-insensitive substring).
func findCharacter(p *data.Plot, name string) *data.Character {
	for i := range p.CharList {
		if p.CharList[i].Name() == name {
			return &p.CharList[i]
		}
	}
	n := strings.ToLower(name)
	for i := range p.CharList {
		if strings.Contains(strings.ToLower(p.CharList[i].Name()), n) {
			return &p.CharList[i]
		}
	}
	return nil
}

// requirePlot finds the plot or returns an error result.
func requirePlot(d *data.Export, title string) (*data.Plot, error) {
	p := d.FindPlot(title)
	if p == nil {
		return nil, fmt.Errorf("plot not found: %s", title)
	}
	return p, nil
}
