#!/usr/bin/env bash
# End-to-end smoke test against a real StoryPlotter backup.
#
# Usage:
#   STORYPLOTTER_DATA_PATH=/path/to/backup.txt ./scripts/smoke_e2e.sh
#
# Exits non-zero if any check fails. Prints a summary either way.

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
check "list_plots"  'Found'                          "$out"
check "search"      'result(s) for'                  "$out"

echo
echo "== http =="
port=18087
MCP_BEARER=devtoken "$bin" -mode=http -addr=":$port" >/dev/null 2>&1 &
pid=$!
# wait for server
for _ in $(seq 1 20); do
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

kill "$pid" 2>/dev/null || true
wait "$pid" 2>/dev/null || true

echo
echo "summary: $pass passed, $fail failed"
exit "$fail"
