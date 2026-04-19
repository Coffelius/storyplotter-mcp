#!/usr/bin/env bash
# End-to-end smoke test against a real StoryPlotter backup.
#
# Usage:
#   STORYPLOTTER_DATA_PATH=/path/to/backup.txt ./scripts/smoke_e2e.sh
#
# Exits non-zero if any check fails. Prints a summary either way.
#
# Covers stdio, HTTP+SSE, and multi-tenant import/export (GAB-92..95).

set -u
set -o pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/.." && pwd)"

: "${STORYPLOTTER_DATA_PATH:?set STORYPLOTTER_DATA_PATH to a StoryPlotter export}"

bin="$repo/bin/storyplotter-mcp"
if [ ! -x "$bin" ]; then
  echo "building $bin"
  (cd "$repo" && go build -o bin/storyplotter-mcp ./cmd/storyplotter-mcp) || exit 1
fi

fail=0
pass=0

check() {
  local name="$1" expect="$2" got="$3"
  if [[ "$got" == *"$expect"* ]]; then
    echo "  ok   $name"
    pass=$((pass+1))
  else
    echo "  FAIL $name — expected to contain: $expect"
    echo "       got: ${got:0:200}"
    fail=$((fail+1))
  fi
}

echo "== stdio =="
out="$(
"$bin" 2>/dev/null <<EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}
{"jsonrpc":"2.0","method":"initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_plots","arguments":{}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search","arguments":{"query":"a","scope":"all"}}}
EOF
)"

check "initialize"  '"protocolVersion":"2024-11-05"' "$out"
check "tools/list"  '"name":"generate_context"'     "$out"
check "tools/list has import_data"  '"name":"import_data"' "$out"
check "tools/list has request_export_link" '"name":"request_export_link"' "$out"
check "list_plots"  'Found'                          "$out"
check "search"      'result(s) for'                  "$out"

echo
echo "== http =="
port=18087
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

signing_key="$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 64)"

MCP_BEARER=devtoken \
  STORYPLOTTER_DATA_DIR="$tmpdir" \
  DOWNLOAD_SIGNING_KEY="$signing_key" \
  MCP_PUBLIC_URL="http://localhost:$port" \
  "$bin" -mode=http -addr=":$port" >/dev/null 2>&1 &
pid=$!
# wait for server
for _ in $(seq 1 30); do
  if curl -sf "http://localhost:$port/healthz" >/dev/null; then break; fi
  sleep 0.1
done

health="$(curl -s http://localhost:$port/healthz)"
check "healthz" '"status":"ok"' "$health"

code="$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:$port/mcp -d '{}')"
check "401 w/o bearer" "401" "$code"

sse="$(curl -s -H "Authorization: Bearer devtoken" -H "Content-Type: application/json" \
  -X POST http://localhost:$port/mcp \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"c","version":"0"}}}')"
check "sse initialize" 'event: message' "$sse"
check "sse done frame" 'event: done'    "$sse"

echo
echo "== multi-tenant =="

mcp_call() {
  # usage: mcp_call <userID-or-empty> <json-rpc-body>
  local uid="$1" body="$2"
  local user_header=()
  if [ -n "$uid" ]; then
    user_header=(-H "X-LibreChat-User-Id: $uid")
  fi
  curl -s -H "Authorization: Bearer devtoken" \
       -H "Content-Type: application/json" \
       "${user_header[@]}" \
       -X POST "http://localhost:$port/mcp" \
       -d "$body"
}

fixture="$repo/testdata/sample.json"
if [ ! -f "$fixture" ]; then
  echo "  FAIL fixture missing at $fixture"
  fail=$((fail+1))
fi
fixture_bytes="$(cat "$fixture")"

# --- Alice imports a fixture ---
# Build the JSON-RPC body with jq to safely embed the fixture string.
if ! command -v jq >/dev/null 2>&1; then
  echo "  skip multi-tenant checks: jq not installed"
else
  import_body="$(jq -cn --arg c "$fixture_bytes" '{jsonrpc:"2.0",id:10,method:"tools/call",params:{name:"import_data",arguments:{content:$c}}}')"
  import_resp="$(mcp_call alice "$import_body")"
  check "alice import_data ok"      'Imported:' "$import_resp"

  # --- Alice sees her plots ---
  alice_list="$(mcp_call alice '{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"list_plots","arguments":{}}}')"
  check "alice list_plots has fixture" 'The Crimson Hour' "$alice_list"

  # --- Bob (no import) has nothing ---
  bob_list="$(mcp_call bob '{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"list_plots","arguments":{}}}')"
  check "bob list_plots empty" 'No plots' "$bob_list"

  # --- Anonymous (no header) hits the shared corpus (14 plots from real backup) ---
  anon_list="$(mcp_call "" '{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"list_plots","arguments":{}}}')"
  check "anon list_plots from shared" 'Found' "$anon_list"

  # --- Alice requests an export link, downloads, and round-trip matches ---
  link_resp="$(mcp_call alice '{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"request_export_link","arguments":{}}}')"
  check "alice export link issued" '/download?t=' "$link_resp"

  # Extract the token from the SSE body. Token charset: base64url + dots
  # (no backslash). grep -o with a strict class avoids swallowing the
  # following `\n` escape inside the JSON-encoded text field.
  token="$(printf '%s' "$link_resp" | grep -oE 't=[A-Za-z0-9_.-]+' | head -1 | sed 's|^t=||')"
  if [ -z "$token" ]; then
    echo "  FAIL could not extract download token"
    fail=$((fail+1))
  else
    dl_body="$(curl -s "http://localhost:$port/download?t=$token")"
    # Compare byte-for-byte with the fixture.
    if [ "$dl_body" = "$fixture_bytes" ]; then
      echo "  ok   download round-trip byte-match"
      pass=$((pass+1))
    else
      echo "  FAIL download round-trip mismatch ($(echo -n "$dl_body" | wc -c) vs $(echo -n "$fixture_bytes" | wc -c) bytes)"
      fail=$((fail+1))
    fi

    # Second GET with the same token should be 410 Gone.
    code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$port/download?t=$token")"
    check "download reused token -> 410" "410" "$code"

    # Tampered token (flip a char in the mac part) should be 401.
    last="${token: -1}"
    alt="a"; [ "$last" = "a" ] && alt="b"
    tampered="${token%?}$alt"
    code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$port/download?t=$tampered")"
    check "download tampered token -> 401" "401" "$code"
  fi

  # --- import_data without user header is rejected ---
  no_uid_resp="$(mcp_call "" "$import_body")"
  check "anon import rejected" 'requires a user identity' "$no_uid_resp"

  # --- Invalid X-LibreChat-User-Id format -> 400 ---
  code="$(curl -s -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer devtoken" \
    -H "X-LibreChat-User-Id: has space!" \
    -H "Content-Type: application/json" \
    -X POST "http://localhost:$port/mcp" -d '{}')"
  check "invalid user-id -> 400" "400" "$code"
fi

kill "$pid" 2>/dev/null || true
wait "$pid" 2>/dev/null || true

echo
echo "summary: $pass passed, $fail failed"
exit "$fail"
