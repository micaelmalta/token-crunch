# LOI Index

Generated: 2026-05-15 (updated)
Source paths: cmd/, internal/

## TASK → LOAD

| Task | Load |
|------|------|
| Understand the full compression pipeline | compress/_root.md |
| Add a new output format / shape detector | compress/core.md |
| Debug dedup false positives or near-match behavior | compress/core.md |
| Tune TF-IDF relevance scoring or token budget | compress/core.md |
| Configure runtime thresholds or enabled strategies | compress/core.md |
| Capture live Claude hook payloads for fixtures | hook/hooks.md |
| Understand the Claude Code hook lifecycle | hook/_root.md |
| Debug why a specific tool output is not being compressed | hook/hooks.md |
| Add support for a new tool response shape (e.g. new MCP tool) | hook/hooks.md |
| Replay a session log to test threshold changes offline | hook/hooks.md |
| Emit replay results as JSON | hook/hooks.md, entrypoints.md |
| Understand session persistence and cross-process cache state | session/_root.md |
| Change the near-match similarity threshold | session/store.md |
| Add new fields to the session entry format | session/store.md |
| Inspect stats, cache metrics, or cleanup session files | session/store.md, entrypoints.md |
| Install or remove hooks in Claude settings (global or --local) | entrypoints.md |
| Validate or inspect the active JSON config file | entrypoints.md |
| Run benchmark scenarios / measure savings | entrypoints.md |
| Explain a single hook payload compression decision | hook/hooks.md, entrypoints.md |
| Understand CLI subcommand routing | entrypoints.md |

## PATTERN → LOAD

| Pattern | Load |
|---------|------|
| Pipeline chain (multi-stage processing) | compress/core.md |
| Content-addressable cache | compress/core.md, session/store.md |
| Near-match diff / LCS similarity | compress/core.md, session/store.md |
| TF-IDF scoring | compress/core.md |
| Shape detection / strategy pattern | compress/core.md |
| Pre/Post/Stop hook lifecycle | hook/hooks.md |
| Atomic file write | session/store.md, entrypoints.md |
| Singleton global store | session/store.md |
| Idempotent merge | entrypoints.md |
| Benchmark harness / fixture-driven | entrypoints.md |

## GOVERNANCE WATCHLIST

No rooms flagged.

## Buildings

| Subdomain | Description | Rooms |
|-----------|-------------|-------|
| compress/ | Three-stage token compression: dedup, structural collapse, relevance trimming | core.md |
| hook/ | Claude Code PreToolUse / PostToolUse / Stop hook handlers and session replay | hooks.md |
| session/ | Content-addressed session store with cross-process persistence and diff utilities | store.md |
| (flat) | CLI entrypoints and settings installer | entrypoints.md |
