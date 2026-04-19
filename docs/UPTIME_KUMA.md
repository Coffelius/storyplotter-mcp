# Uptime Kuma monitor for StoryPlotter MCP

Target: `https://status.gabi.tv` — the same Uptime Kuma instance set up in GAB-13, already wired to the Telegram channel from GAB-56.

## Monitor fields

Create a new **HTTP(s) – Keyword** monitor with these values:

| Field                | Value                                                   |
|----------------------|---------------------------------------------------------|
| Monitor Type         | `HTTP(s) - Keyword`                                     |
| Friendly Name        | `StoryPlotter MCP`                                      |
| URL                  | `https://mcp-storyplotter.gabi.tv/healthz`              |
| Keyword              | `"status":"ok"`                                         |
| Invert Keyword       | off                                                     |
| Heartbeat Interval   | `60` seconds                                            |
| Retries              | `2` before marking DOWN                                 |
| Heartbeat Retry Int. | `20` seconds                                            |
| Request Timeout      | `10` seconds                                            |
| HTTP Method          | `GET`                                                   |
| Accepted Status      | `200-299`                                               |
| Max. Redirects       | `0`                                                     |
| Ignore TLS Errors    | off (Let's Encrypt via Traefik, should be valid)        |
| Upside Down Mode     | off                                                     |

No `Authorization` header — `/healthz` is intentionally unauthenticated so monitors can hit it without sharing the bearer.

## Notifications

In the monitor's **Notifications** tab, enable the existing Telegram channel (`@gabiCoolBot`, chat `1084015735`) that was configured in GAB-56. No new notification provider is needed.

## Tag

Add a tag `mcp` so all MCP monitors (this one and any future ones) can be grouped on the dashboard.

## Verification

After saving, the monitor should go green within one interval. Confirm manually:

```bash
curl -sf https://mcp-storyplotter.gabi.tv/healthz
# -> {"status":"ok"}
```

Then simulate a failure once by stopping the Coolify service — you should receive a Telegram alert within ~2 × 60s (retries × interval) and a recovery alert after bringing it back.

## Why keyword, not plain HTTP

A plain 200-status check would pass even if the binary started without data or the handler regressed to an empty body. The keyword variant ensures we're actually getting the expected JSON, which is what LibreChat needs to believe before trusting the MCP.

## Related tickets

- [GAB-85 — Uptime Kuma monitor + Telegram](https://linear.app/gabistuff/issue/GAB-85)
- [GAB-13 — Uptime Kuma deploy](https://linear.app/gabistuff/issue/GAB-13)
- [GAB-56 — Coolify Telegram notifications](https://linear.app/gabistuff/issue/GAB-56)
- [GAB-83 — Coolify sidecar deploy (prerequisite)](https://linear.app/gabistuff/issue/GAB-83)
