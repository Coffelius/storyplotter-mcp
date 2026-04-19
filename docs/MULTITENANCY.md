# Multi-tenant storage model

StoryPlotter MCP can serve per-user corpora when deployed behind LibreChat.
Each LibreChat user gets a private `storyplotter.json` on the MCP's persistent
volume and an aisle-isolated view through every tool call.

## Trust model

Two layers of auth, each playing a distinct role:

- **`Authorization: Bearer ${MCP_BEARER}`** — authenticates LibreChat (the
  client) as a whole. One shared secret, rotatable in Coolify.
- **`X-LibreChat-User-Id: {{LIBRECHAT_USER_ID}}`** — identifies the human
  behind the call. LibreChat substitutes the placeholder on every request;
  the header is already validated on LibreChat's side as the authenticated
  user's Mongo ObjectId.

The MCP **trusts LibreChat** with identity. It does not verify per-user
tokens. Any caller that presents a valid Bearer is taken as "LibreChat
legitimately speaking for whomever the `X-LibreChat-User-Id` names". This
suffices for the `chat.gabi.tv` scale (owner + a handful of friends). If
the deployment is ever opened to untrusted public users, migrate to the
OAuth2 provider path sketched in
[`OAUTH2_MIGRATION.md`](OAUTH2_MIGRATION.md).

## Storage layout

```
inside the container (via volume mount):
  /data/users/<libreChatUserId>/storyplotter.json   # per-user corpus
  /data/storyplotter.json                           # shared fallback (read-only)

on the Coolify host:
  /data/coolify/applications/<mcp-app-uuid>/users/<libreChatUserId>/storyplotter.json
```

- `<libreChatUserId>` is Mongo's ObjectId for the user (stable across
  sessions, chats, and deploys).
- Directories are `0700`, files `0600`.
- Writes are atomic (`os.CreateTemp` in the same directory + `os.Rename`);
  a crash mid-write leaves the previous version intact.
- A small LRU cache (default 50 users) holds recently-loaded `*Export`
  instances in memory; invalidated on every `Save`/`Replace`.

## Request flow

```
LibreChat POST https://mcp-storyplotter.gabi.tv/mcp
  Authorization: Bearer ${MCP_BEARER}
  X-LibreChat-User-Id: 6543f9c4e2ab1d0012b3e481
  (body: JSON-RPC tool call)

   └─► bearerMiddleware        — rejects missing/wrong Bearer (401).
       └─► userContextMiddleware — extracts + validates X-LibreChat-User-Id
           (regex `^[a-zA-Z0-9_-]{1,64}$`; malformed → 400); stashes
           user id on request context.
           └─► rate limit (60 req/min per uid by default)
               └─► body-size cap (5 MB by default)
                   └─► serveMCP → Dispatch(req, r)
                       └─► tool Handler(args, *CallContext)
                           cc.UserID = "6543f9c4e2ab1d0012b3e481"
                           cc.Store.Load(userID)
                           → /data/users/6543f9c4e.../storyplotter.json
```

Missing `X-LibreChat-User-Id` is **not** an error. It falls through to the
shared read-only corpus (legacy single-file deploy). Writes are rejected in
that mode.

## Import flow (chat attachment)

1. User drags their Story Plotter export (`.txt`/`.json`) into a LibreChat
   conversation with a tool-capable model.
2. LibreChat attaches the file to the prompt; the model reads the text.
3. The model calls `import_data({ content: "<entire file>" })`.
4. MCP validates with `data.Parse`; if it parses, writes the raw bytes
   atomically to `/data/users/<uid>/storyplotter.json` (preserving original
   formatting).
5. Responds with `"Imported: N plots, M characters across all plots, K eras,
   F folders."`.
6. Subsequent tool calls for the same user see the new corpus.

The whole file round-trips through the LLM context once. For a typical
60 KB export that's ~15 K tokens — a one-time cost absorbed by the chat.

## Export flow (signed download link)

1. User asks for a copy of their StoryPlotter.
2. The model calls `request_export_link()`.
3. MCP generates an HMAC-signed token (TTL 5 min, single use) and responds
   with `"Download link (expires in 5 min): https://mcp-storyplotter.gabi.tv/download?t=<token>"`.
4. User clicks / opens the link in the browser.
5. MCP's `/download` handler verifies the token, consumes the nonce, and
   streams the raw `storyplotter.json` with
   `Content-Disposition: attachment; filename="storyplotter-YYYYMMDD-HHMM.json"`.
6. Reusing the same token returns 410 Gone. A tampered token returns 401.

The `/download` route bypasses Bearer (the signed token is the auth) and
is rate-limited by client IP (30 req/min by default).

## Required Coolify env

```
# Identity & auth
MCP_BEARER=<shared with LibreChat service>
DOWNLOAD_SIGNING_KEY=<openssl rand -hex 32>
MCP_PUBLIC_URL=https://mcp-storyplotter.gabi.tv

# Storage
STORYPLOTTER_DATA_DIR=/data/users
STORYPLOTTER_DATA_PATH=/data/storyplotter.json   # optional; shared fallback

# Hardening (tuneable; defaults shown)
MCP_BODY_LIMIT_BYTES=5242880
MCP_RATE_LIMIT_PER_MIN=60
DOWNLOAD_RATE_LIMIT_PER_MIN=30
```

## LibreChat config

In `~/Projects/coolify-infra/chatbot/librechat.yaml` under `mcpServers`:

```yaml
storyplotter:
  type: streamable-http
  url: https://mcp-storyplotter.gabi.tv/mcp
  headers:
    Authorization: "Bearer ${MCP_BEARER}"
    X-LibreChat-User-Id: "{{LIBRECHAT_USER_ID}}"
    X-LibreChat-User-Email: "{{LIBRECHAT_USER_EMAIL}}"
  description: "StoryPlotter narrative database (per user)."
  timeout: 60000
```

The `{{LIBRECHAT_USER_*}}` placeholders are resolved **per request** by
LibreChat ≥ v0.8.2. Same deployment pattern used by the Notion / Linear
hosted MCPs, except those use OAuth2 for identity.

## Fallback for anonymous / stdio callers

Any caller without the `X-LibreChat-User-Id` header (smoke tests, curl,
stdio mode, local dev) sees the legacy shared corpus at
`STORYPLOTTER_DATA_PATH` in **read-only** mode. `import_data` and
`request_export_link` both refuse to operate without a user identity. This
preserves the pre-GAB-92 behaviour for anyone who was already pointing a
client at the bare `/mcp` endpoint.

## Verification

`scripts/smoke_e2e.sh` exercises the full lifecycle locally. It spins up
the binary in HTTP mode with a tmpdir data root and runs:

- stdio: initialize → tools/list (includes `import_data` + `request_export_link`) → `list_plots` → `search`.
- HTTP: `/healthz`, 401 without Bearer, SSE `event: message` + `event: done`.
- Multi-tenant: alice imports → alice sees her data → bob sees nothing →
  anon sees shared corpus → alice's export link round-trips byte-identical
  → second GET with same token is 410 → tampered token is 401 → anon
  `import_data` is rejected → invalid `X-LibreChat-User-Id` format is 400.

Current state of the suite: 20/20 passing against the real backup.

## Related Linear tickets

- [GAB-92 — header + CallContext](https://linear.app/gabistuff/issue/GAB-92)
- [GAB-93 — DiskUserStore + atomic writes](https://linear.app/gabistuff/issue/GAB-93)
- [GAB-94 — import_data](https://linear.app/gabistuff/issue/GAB-94)
- [GAB-95 — request_export_link + /download](https://linear.app/gabistuff/issue/GAB-95)
- [GAB-96 — docs + smoke e2e (this document)](https://linear.app/gabistuff/issue/GAB-96)
- [GAB-97 — body cap + rate limit](https://linear.app/gabistuff/issue/GAB-97)
- [GAB-98 — OAuth2 follow-up evaluation](https://linear.app/gabistuff/issue/GAB-98)
