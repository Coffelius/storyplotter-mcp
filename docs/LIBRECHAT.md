# Wiring StoryPlotter MCP into LibreChat

Target host: `chat.gabi.tv` (LibreChat on Coolify — see the `n8n + Chatbot on Coolify` Linear project).

Prerequisite: the MCP is deployed at `https://mcp-storyplotter.gabi.tv` with a Bearer token — follow `docs/DEPLOY_COOLIFY.md` first.

## 1. Add the bearer to LibreChat's env

In Coolify, edit the LibreChat service environment:

```
MCP_BEARER=<same value used when deploying the MCP>
```

Redeploy so the env propagates.

## 2. Paste the MCP into `librechat.yaml`

Add the block from `examples/librechat-snippet.yaml` under the top-level `mcpServers:` key. If no other MCPs are configured yet, create the key:

```yaml
mcpServers:
  storyplotter:
    type: streamable-http
    url: https://mcp-storyplotter.gabi.tv/mcp
    headers:
      Authorization: "Bearer ${MCP_BEARER}"
```

Commit the config to whichever source of truth Coolify reads from (either a mounted file or the service's config editor) and redeploy LibreChat.

## 3. Smoke test from the VPS

From the Coolify host (or anywhere with outbound HTTPS):

```bash
curl -sf https://mcp-storyplotter.gabi.tv/healthz
# -> {"status":"ok"}

curl -s \
  -H "Authorization: Bearer $MCP_BEARER" \
  -H "Content-Type: application/json" \
  -X POST https://mcp-storyplotter.gabi.tv/mcp \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
# -> event: message ... (9 tools) ... event: done
```

## 4. Validate from the chatbot UI

Open `chat.gabi.tv`, start a new chat with a tool-capable model (Claude Sonnet / Opus, GPT-4+, GLM-4.7), and ask:

> List the StoryPlotter plots I have, then generate a scene for the one about Killer using 2000 tokens of context.

The model should pick `list_plots` → `generate_context` without being told tool names. Confirm in the Linear ticket (GAB-86) that the transcript shows both calls.

## Compatibility note

LibreChat's MCP schema has evolved across releases. If `type: streamable-http` is rejected, try `type: sse` — our server returns the same SSE framing for both. If the UI doesn't show the tools after reload, check LibreChat's logs for `mcp` entries (usually `Failed to initialize` with the underlying error).

## Related tickets

- [GAB-84 — LibreChat integration](https://linear.app/gabistuff/issue/GAB-84)
- [GAB-18 — MCPs into LibreChat (parent)](https://linear.app/gabistuff/issue/GAB-18)
- [GAB-86 — E2E validation](https://linear.app/gabistuff/issue/GAB-86)
