package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Coffelius/storyplotter-mcp/internal/mcp"
)

const defaultPublicURL = "https://mcp-storyplotter.gabi.tv"

// RequestExportLink returns the request_export_link tool. It mints a
// short-lived, single-use download URL for the caller's JSON corpus.
func RequestExportLink() mcp.Tool {
	return mcp.Tool{
		Def: mcp.ToolDefinition{
			Name:        "request_export_link",
			Description: "Mint a one-shot HTTPS download link (expires in 5 minutes) for the caller's StoryPlotter JSON corpus. Requires user identity.",
			InputSchema: schema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		Handler: func(_ json.RawMessage, cc *mcp.CallContext) (mcp.CallToolResult, error) {
			if cc.UserID == "" {
				return mcp.ErrorResult("request_export_link requires a user identity"), nil
			}
			if cc.Signer == nil {
				return mcp.ErrorResult("request_export_link is HTTP-only; this server was started without a signing key"), nil
			}
			raw, _ := cc.Store.Raw(cc.UserID)
			if len(raw) == 0 {
				return mcp.ErrorResult("no data imported yet — use import_data first"), nil
			}
			token := cc.Signer.Sign(cc.UserID, 5*time.Minute)
			base := cc.PublicURL
			if base == "" {
				base = defaultPublicURL
			}
			msg := fmt.Sprintf(
				"Download link (expires in 5 min): %s/download?t=%s\nPaste it in your browser to download storyplotter.json.",
				base, token,
			)
			return mcp.TextResult(msg), nil
		},
	}
}
