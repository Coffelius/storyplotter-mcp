# E2E validation — GAB-86

Real-backup smoke run against a local StoryPlotter export.

## Setup

- Binary: `bin/storyplotter-mcp` (built from `develop`)
- Data: `~/Projects/StoryPlotter/StoryPlotter_BackUp_sat, Jan_31_2026.txt` (14 plots, loaded from the author's personal archive)
- Runner: `scripts/smoke_e2e.sh`

## Result

```
== stdio ==
  ok   initialize
  ok   tools/list
  ok   list_plots
  ok   search

== http ==
  ok   healthz
  ok   401 w/o bearer
  ok   sse initialize
  ok   sse done frame

summary: 8 passed, 0 failed
```

Server logs confirm the loader parses the outer envelope plus the 4 nested JSON strings:

```
[storyplotter-mcp] loaded 14 plots from ...StoryPlotter_BackUp_sat, Jan_31_2026.txt
```

## Tool sample: `generate_context`

Invocation (stdio):

```json
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{
  "name":"generate_context",
  "arguments":{
    "plot":"Jul/16/2025 23:34",
    "focus":"Write a scene with Killer",
    "maxTokens":2000
  }
}}
```

Returned a 1668-char structured context with sections: `# System`, `# Focus`, `# Plot`, `# Character Profiles` (populated from the real `killer au` character's `charParam` fields). This is the output shape LibreChat will feed to the model for Fase 3 use cases.

## Notes

- Plot titles in the real data are timestamped strings (e.g. `Created at wed, Jul/16/2025 23:34`). `get_plot` correctly matches by case-insensitive substring — searching for `"fnaf"` returned nothing because that string is a folder path, not a title. Consider a follow-up to let `get_plot` also match on `folderPath` for a friendlier UX.
- `search` works across titles, character names, sequence cards, relationships, eras, events, and areas.
- HTTP SSE frames follow `event: message` → `event: done` as expected by clients like LibreChat.

## How to re-run

```bash
go build -o bin/storyplotter-mcp ./cmd/storyplotter-mcp
STORYPLOTTER_DATA_PATH="/path/to/your/backup.txt" ./scripts/smoke_e2e.sh
```
