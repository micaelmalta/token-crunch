# token-crunch — Campus Map

Context-aware token compression for Claude Code. Intercepts tool outputs via Claude hooks (PreToolUse / PostToolUse / Stop), compresses them in-process, and persists savings across turns using a content-addressed session store.

## Buildings

| Directory | Owns |
|:----------|:-----|
| [compress/](compress/_root.md) | Three-strategy compression pipeline: differential dedup, structural collapse, TF-IDF relevance trim |
| [session/](session/_root.md) | Content-addressed in-memory + disk session store; LCS-based diff ratio and unified diff |
| [hook/](hook/_root.md) | Claude hook entrypoints (pre/post/flush), auto-compact nudge, explain and replay diagnostics |
| [entrypoints.md](entrypoints.md) | CLI main, savings benchmark, runtime config, settings.json installer |

## Key flows

- **Compression path:** `hook/post.go` → `compress/pipeline.go` → dedup / structure / relevance → `session/store.go`
- **Cache-hit path:** `hook/pre.go` looks up tool-input hash in session store → injects cached result via `additionalContext`
- **Persistence:** `hook/flush.go` (Stop hook) increments turn, records context-window usage, flushes store to `~/.local/share/token-crunch/`
- **Install:** `internal/install/install.go` merges hooks into `~/.claude/settings.json` non-destructively

## TASK → LOAD

| Task | Load |
|------|------|
| Understand the full compression pipeline | compress/_root.md |
| Add a new output format / shape detector | compress/core.md |
| Debug dedup false positives or near-match behavior | compress/core.md |
| Tune TF-IDF relevance scoring or token budget | compress/core.md |
| Configure runtime thresholds or enabled strategies | entrypoints.md |
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
| Auto-compact nudge injection | hook/hooks.md |
