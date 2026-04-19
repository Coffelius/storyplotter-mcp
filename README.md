# storyplotter-mcp

Model Context Protocol server (Go) that exposes a
[StoryPlotter](https://www.storyplotter.jp/) JSON export as tools for LLM
clients like [LibreChat](https://www.librechat.ai/). Deployed at
`https://mcp-storyplotter.gabi.tv` on the author's Coolify host.

## Tools

Read-only:

- `list_plots(folder?, status?, tag?)`
- `get_plot(title)` — title match → folder fallback with disambiguation.
- `list_characters(plot?, priority?)`
- `get_character(plot, name)`
- `list_relationships(plot, character?)`
- `list_eras(plot)` / `list_events(plot, era?)`
- `search(query, scope?)`
- `generate_context(plot, focus, targetCharacters?, maxTokens?)` —
  structured writing-assistant context.

Write / export:

- `import_data(content, overwrite?)` — LibreChat user pastes/attaches a
  Story Plotter export; the MCP stores it under their identity.
- `request_export_link()` — returns a short-lived signed HTTPS URL the
  user opens in the browser to download `storyplotter.json`.

## Storage

Per-user under `/data/users/<libreChatUserId>/storyplotter.json`; falls
back to a shared read-only corpus at `/data/storyplotter.json` for
callers without a `X-LibreChat-User-Id` header. Full model +
deployment details in [`docs/MULTITENANCY.md`](docs/MULTITENANCY.md).

## Local run

```bash
# stdio (single-corpus, no auth)
STORYPLOTTER_DATA_PATH=/path/to/export.json \
  go run ./cmd/storyplotter-mcp

# http (per-user, full stack)
MCP_BEARER=$(openssl rand -hex 16) \
  DOWNLOAD_SIGNING_KEY=$(openssl rand -hex 32) \
  STORYPLOTTER_DATA_DIR=/tmp/users \
  STORYPLOTTER_DATA_PATH=/path/to/export.json \
  MCP_PUBLIC_URL=http://localhost:8080 \
  go run ./cmd/storyplotter-mcp -mode=http -addr=:8080
```

## Smoke test

```bash
STORYPLOTTER_DATA_PATH=/path/to/export.json ./scripts/smoke_e2e.sh
```

20 checks covering stdio, HTTP+SSE, and the full multi-tenant
import/export round-trip.

## Deploy

See [`docs/DEPLOY_COOLIFY.md`](docs/DEPLOY_COOLIFY.md) for the Coolify
sidecar setup and [`docs/LIBRECHAT.md`](docs/LIBRECHAT.md) for wiring
into LibreChat.
