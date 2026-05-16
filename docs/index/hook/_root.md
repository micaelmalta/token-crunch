# Hook Building

Subdomain: hook/
Source paths: internal/hook/

Claude Code hook entrypoints wired into the Claude settings hooks pipeline. Each hook call is a short-lived process: Pre looks up the tool-input cache and injects results as additionalContext; Post runs the compression pipeline and returns updatedToolOutput; Flush (Stop) increments the turn counter and persists the store. Explain and Replay support offline diagnostics without a live Claude session.

## TASK → LOAD

| Task | Load |
|------|------|
| Understand Claude Code hook lifecycle (pre/post/stop) | hooks.md |
| Debug why a tool output is not being compressed | hooks.md |
| Understand how tool response JSON is parsed across tool types | hooks.md |
| Replay a session log to test threshold changes | hooks.md |
| Add support for a new tool response shape | hooks.md |
| Understand auto-compact nudge triggering | hooks.md |
| Capture raw hook payloads to disk for fixture creation | hooks.md |

## Rooms

| Room | Source paths | Files |
|------|-------------|-------|
| hooks.md | internal/hook/ | pre.go, post.go, flush.go, compact.go, explain.go, replay.go, capture.go, debug.go |
