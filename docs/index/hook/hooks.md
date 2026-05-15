---
room: hook/hooks
see_also:
  - compress/core.md
  - session/store.md
hot_paths:
  - hook/post.go → Post
  - hook/pre.go → Pre
architectural_health: normal
security_tier: normal
---

# hook/flush.go

DOES: Handles the Claude Code Stop hook by optionally capturing the raw hook payload, incrementing the session turn counter, and flushing the in-memory store to disk so state survives across hook process boundaries.
SYMBOLS:
- Flush() error
- Types: flushInput { SessionID }
DEPENDS: internal/session
PATTERNS: stop-hook, flush-to-disk

---

# hook/post.go

DOES: Handles the PostToolUse hook by optionally capturing the raw hook payload, extracting tool output text from Bash/Read/MCP/generic/nested shapes, skipping binary-looking huge lines, running the compression pipeline, storing a tool-input cache entry, and returning the compressed output via updatedToolOutput when the original response shape can be preserved.
SYMBOLS:
- Post() error
- postWithStore(store *session.Store, inp postInput, w io.Writer, persist bool) error
- toolText(raw json.RawMessage) string
- collectText(v any) []string
- isProbablyBinaryOrHugeLine(s string) bool
- updatedToolOutput(raw json.RawMessage, replacement string) (any, bool)
- isError(raw json.RawMessage) bool
- Types: postInput { SessionID, ToolName, ToolInput, ToolResponse }
DEPENDS: internal/compress, internal/session
PATTERNS: post-hook, adapter

---

# hook/pre.go

DOES: Handles the PreToolUse hook by optionally capturing the raw hook payload and looking up the tool's cache key in the session store; on a hit it injects the stored result as additionalContext so the model sees the cached result before the tool runs. The store is NOT flushed on a cache miss (no disk write on the hot miss path).
SYMBOLS:
- Pre() error
- buildCacheKey(toolName string, input map[string]any) string
- Types: preInput { SessionID, ToolName, ToolInput }
DEPENDS: internal/session
PATTERNS: pre-hook, cache-lookup

---

# hook/explain.go

DOES: Reads a hook payload or replay entry, runs it through the compression pipeline with an ephemeral store, and prints a JSON report with applied strategies, decision trace, and output.
SYMBOLS:
- Explain(path string) error
- Types: ExplainReport { ToolName, Original, Final, Modified, Strategies, Trace, Output }
DEPENDS: internal/compress, internal/session
PATTERNS: offline-analysis, debug-explain

---

# hook/replay.go

DOES: Reads a JSONL session log and replays each entry through the compression pipeline with current settings, printing either a per-entry savings table or JSON report for threshold tuning.
SYMBOLS:
- Replay(logPath string) error
- ReplayJSON(logPath string) error
- collectReplay(logPath string) (ReplayReport, error)
- newEphemeralStore() *session.Store
- Types: ReplayEntry { ToolName, ToolInput, Content }, ReplayReport { Entries, Total }
DEPENDS: internal/compress, internal/session
PATTERNS: replay, offline-analysis
USE WHEN: Tuning compression thresholds or validating a new strategy against a recorded session.

---

# hook/capture.go

DOES: Writes raw hook stdin payloads to `TOKEN_CRUNCH_CAPTURE_DIR` using safe timestamped filenames for live fixture capture.
SYMBOLS:
- capturePayload(event string, raw []byte)
- safeEventName(event string) string
DEPENDS: os, filepath, time
PATTERNS: fixture-capture

---

# hook/debug.go

DOES: Emits hook decision logs to stderr when `TOKEN_CRUNCH_DEBUG=true`.
SYMBOLS:
- debugf(format string, args ...any)
DEPENDS: internal/config
PATTERNS: debug-logging
