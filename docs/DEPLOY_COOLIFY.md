# Deploying StoryPlotter MCP to Coolify

Target host: the existing Contabo VPS running Coolify (see the
`n8n + Chatbot on Coolify` Linear project). This guide follows the
"sidecar Docker in Coolify" path decided in GAB-16.

**Nothing here runs automatically** — execute the steps when you're in front of the Coolify UI.

## 0. Prerequisites

- Coolify up with Traefik + Let's Encrypt (already true).
- A subdomain pointing at the VPS. Suggested: `mcp-storyplotter.gabi.tv`.
  Add an A record to the same IP as `chat.gabi.tv`.
- SSH / Tailscale access to the host (for uploading the JSON export).
- A StoryPlotter JSON backup on your machine.

## 1. Push the repo to GitHub

Coolify deploys from Git. This MCP repo has no remote yet; once you create `github.com/gabistuff/storyplotter-mcp` (public or private, either works), push:

```bash
git remote add origin git@github.com:gabistuff/storyplotter-mcp.git
git push -u origin main
git push -u origin develop
```

(You can skip this and use Coolify's "docker image" flow instead, but Git is the path already established by the other services on this host.)

## 2. Create the Coolify application

In Coolify → Projects → your project → New Resource → **Application**:

- **Source:** GitHub (use the existing `Gabistuff` Git source configured in GAB-64).
- **Repo:** `gabistuff/storyplotter-mcp`.
- **Branch:** `main` (set up promotion from `develop` via a PR flow once
  the repo is public; for an early deploy, `develop` is fine).
- **Build pack:** Dockerfile (the repo's `Dockerfile` is multi-stage
  and produces an Alpine image on :8080).

## 3. Runtime configuration

In the app's settings:

- **Port:** `8080` exposed.
- **Domain:** `mcp-storyplotter.gabi.tv`, HTTPS on, Let's Encrypt.
- **Healthcheck:** `GET /healthz` every 30s.
- **Environment variables:**
  - `STORYPLOTTER_DATA_PATH=/data/storyplotter.json`
  - `MCP_BEARER=<generate with: openssl rand -hex 32>` — mark as secret.
- **Persistent volume:** host path `/opt/storyplotter-mcp/data` → container path `/data`. Read-only is fine.

## 4. Upload the JSON export

From your laptop:

```bash
export COOLIFY_SSH=coolify-prod            # Tailscale shortname configured in GAB-57
./scripts/upload_data.sh ~/Projects/StoryPlotter/StoryPlotter_BackUp_sat,\ Jan_31_2026.txt
```

The script:

- Creates `/opt/storyplotter-mcp/data` on the host if missing.
- `rsync`s the backup in as `storyplotter.json`.
- Verifies the file landed and is readable.

After upload, restart the Coolify service so the loader picks up the file.

## 5. Post-deploy verification

Run the same smoke checks `scripts/smoke_e2e.sh` runs locally, against the remote URL:

```bash
curl -sf https://mcp-storyplotter.gabi.tv/healthz
# -> {"status":"ok"}

curl -s -H "Authorization: Bearer $MCP_BEARER" \
     -H "Content-Type: application/json" \
     -X POST https://mcp-storyplotter.gabi.tv/mcp \
     -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
# -> event: message ... event: done
```

Expected server log line on first request: `loaded N plots from /data/storyplotter.json`.

## 6. Hook up the rest of the stack

- **LibreChat:** see `docs/LIBRECHAT.md` (GAB-84).
- **Uptime Kuma:** add a Monitor at `https://mcp-storyplotter.gabi.tv/healthz`, reuse the Telegram alert channel from GAB-56 (GAB-85).
- **Coolify Telegram:** already configured (GAB-56) — deploy failures will notify automatically.

## 7. Updating the data

Repeat step 4 and restart the service whenever the export changes. A future follow-up: have the StoryBridge viewer (`story-plotter-viewer`) push exports directly over a separate write-auth endpoint. Out of scope for v1.

## Rollback

If a deploy breaks:

```
Coolify UI → the app → Deployments → click the last known-good build → Redeploy
```

The persistent volume (the JSON) is not affected by container rollback.

## Related tickets

- [GAB-83 — Coolify sidecar deploy](https://linear.app/gabistuff/issue/GAB-83)
- [GAB-16 — MCP architecture decision (sidecar path)](https://linear.app/gabistuff/issue/GAB-16)
- [GAB-64 — GitHub as Git source (Coolify)](https://linear.app/gabistuff/issue/GAB-64)
- [GAB-57 — Tailscale SSH](https://linear.app/gabistuff/issue/GAB-57)
