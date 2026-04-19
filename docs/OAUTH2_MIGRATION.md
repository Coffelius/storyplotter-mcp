# OAuth2 Migration Evaluation (GAB-98)

Status: evaluation only. No code changes. Intended for the repo maintainer in ~6 months deciding whether to migrate away from the current "trust the `X-LibreChat-User-Id` header" model.

## Why this doc exists

The StoryPlotter MCP currently authenticates inbound `/mcp` traffic with a single static `MCP_BEARER` token. That Bearer authenticates *LibreChat as a client*, not the human behind the chat. User identity is carried in-band via an `X-LibreChat-User-Id` HTTP header that the MCP trusts verbatim to key per-user storage under `/data/users/<libreChatUserId>/storyplotter.json`.

This model is appropriate for the current deployment shape: a single owner plus a handful of known-trusted friends, all logging into one LibreChat instance that the owner also operates. The security boundary is effectively "own the LibreChat admin" = "own everything downstream".

It fails open in two well-defined scenarios:

1. **LibreChat compromise.** Any attacker that gains code execution on the LibreChat side can set the header to any value and read/write any user's story data.
2. **Bearer leak without LibreChat compromise.** An attacker with `MCP_BEARER` can hit `/mcp` directly and spoof `X-LibreChat-User-Id` to any value. Mitigated somewhat because the Bearer is rotatable and the MCP is not publicly advertised, but the header itself is unauthenticated.

Neither is catastrophic today. Both become unacceptable the moment the trust assumptions above change.

## Decision triggers (when to migrate)

Migrate when **any one** of these becomes true:

- LibreChat registration opens to untrusted public users (self-serve signup without vetting).
- The user base grows beyond ~3 semi-trusted users.
- Regulatory exposure appears: EU residents' personal data, third-party PII, or contractual data-handling obligations.
- `MCP_BEARER` leaks (rotation buys time but is not a fix).

Until one of those fires, the current model is fine and migration is wasted effort.

## Target architecture

OAuth2 Authorization Code + PKCE, with the **MCP server itself acting as the OAuth2 provider**. LibreChat's MCP client (v0.8.2+) performs RFC 8414 discovery against `.well-known/oauth-authorization-server` after receiving a `401` from `/mcp`. See `danny-avila/LibreChat#8049` for client-side implementation status and the [LibreChat MCP docs](https://www.librechat.ai/docs/features/mcp) for configuration shape.

Per-user access tokens are JWTs with a short TTL (~15 min). Refresh tokens optional but recommended for UX. A single scope (`storyplotter:rw`) is enough to start; finer-grained scopes are out of scope (see below). The JWT `sub` claim replaces `X-LibreChat-User-Id` as the per-user identity key.

Minimal discovery document shape:

```json
{
  "issuer": "https://mcp-storyplotter.gabi.tv",
  "authorization_endpoint": "https://mcp-storyplotter.gabi.tv/oauth/authorize",
  "token_endpoint": "https://mcp-storyplotter.gabi.tv/oauth/token",
  "code_challenge_methods_supported": ["S256"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "scopes_supported": ["storyplotter:rw"]
}
```

## Implementation effort (three paths)

- **A. Hand-rolled** using stdlib + `golang.org/x/oauth2`. ~400-600 LoC. Full control, minimal deps. High risk of mis-implementing PKCE verifier comparison, state/CSRF handling, or the token-endpoint error shape. **1-2 days** for a working prototype, but likely another week of review/hardening before production. Not recommended unless the maintainer wants the learning exercise.
- **B. `github.com/ory/fosite`.** Full RFC coverage including introspection and revocation. ~200-300 LoC integration code but a heavy dep surface, pluggable storage, and a steeper conceptual curve. Overkill for current scale. **3-5 days.**
- **C. `github.com/go-oauth2/oauth2/v4`.** Lighter middle ground. Enough RFC coverage for our single-client, single-scope case; well-trodden. ~300-400 LoC integration. **2-3 days.** **Likely default choice.**

## Data migration

Current storage path: `/data/users/<libreChatUserId>/storyplotter.json`. Post-migration the path is keyed by `sub`. Three options:

- **(a) Use LibreChat's user id as `sub`.** Zero data movement. Requires the provider to either reuse LibreChat's identity or mint tokens whose `sub` matches. **Recommended.**
- (b) One-shot migration script that renames directories old→new once a mapping exists.
- (c) Require users to re-import their stories.

Option (a) is the cheapest and removes an entire class of migration failure modes.

## Operational cost

- **JWT signing-key rotation.** `kid` header plus a small key registry; keep the previous key valid for one TTL window to avoid mid-request invalidation.
- **Revocation.** In-memory revocation list, periodic dump to disk, reload on boot. No need for a full blacklist service at this scale.
- **Grant storage.** SQLite in a mounted volume is simplest. Reusing LibreChat's MongoDB (already in the stack) is tempting but couples deploys.
- **Testing.** Stand the new MCP up against the LibreChat staging instance and run the full tool surface before flipping prod.

## Out of scope

- Social login (Google/GitHub) on the MCP itself. LibreChat already handles human login; the MCP only needs to trust its authorization decisions.
- Per-tool scopes. A single `storyplotter:rw` is sufficient until a real authorization story exists.
- Multi-tenancy beyond LibreChat-as-client. No third-party MCP clients are planned.

## Recommended migration sequence

Pragmatic checklist for when a trigger above fires:

1. Fork a branch off the current MCP.
2. Add OAuth2 provider code (option C, `go-oauth2/oauth2/v4`).
3. Deploy as a parallel MCP at `mcp-storyplotter-oauth.gabi.tv`.
4. Update LibreChat config to point at the new URL; remove the Bearer + header auth entry.
5. Observe for one week (logs, error rates, per-user traffic).
6. Retire the old Bearer endpoint.
