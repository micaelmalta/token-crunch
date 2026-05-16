---
room: hook/hooks
see_also:
  - compress/core.md
  - session/store.md
hot_paths:
  - hook/post.go → Post
  - hook/pre.go → Pre
  - hook/flush.go → Flush
architectural_health: normal
security_tier: normal
---

# hook/capture.go

DOES: Writes raw hook stdin payloads to `TOKEN_CRUNCH_CAPTURE_DIR` using safe timestamped filenames (UTC, nanosecond precision) so live Claude sessions can generate fixture files for tests.
SYMBOLS:
- capturePayload(event string, raw []byte)
- safeEventName(event string) string
PATTERNS: fixture-capture

---

# hook/compact.go

DOES: Checks whether the context-window usage percentage recorded by the last Stop hook exceeds the configured CompactThreshold and, if so, injects a one-time compaction nudge message into additionalContext (guarded by CompactRequested to prevent duplicate nudges within the same turn).
SYMBOLS:
- compactContext(store *session.Store, cfg config.Config) string
DEPENDS: internal/config, internal/session
PATTERNS: auto-compact-nudge
USE WHEN: Changing the nudge message wording; adjusting when auto-compact fires relative to context fill percentage.

---

# hook/debug.go

DOES: Emits hook decision log lines to stderr only when `TOKEN_CRUNCH_DEBUG=true` is set in the environment.
SYMBOLS:
- debugf(format string, args ...any)
DEPENDS: internal/config
PATTERNS: debug-logging

---

# hook/explain.go

DOES: Reads a hook payload (PostToolUse JSON) or a ReplayEntry from a file or stdin, runs it through the compression pipeline with a fresh ephemeral store, and prints a JSON report detailing applied strategies, decision trace steps, original/final sizes, and the compressed output.
SYMBOLS:
- Explain(path string) error
- readInput(path string) ([]byte, error)
- printExplain(toolName string, toolInput map[string]any, content string) error
TYPE: ExplainReport { ToolName string, Original int, Final int, Modified bool, Strategies []string, Trace []string, Output string }
DEPENDS: internal/compress, internal/session
PATTERNS: offline-analysis, debug-explain
USE WHEN: Diagnosing why a specific payload was or was not compressed; validating strategy selection for a new fixture.

---

# hook/flush.go

DOES: Handles the Claude Code Stop hook — parses the context-window usage percentage from the payload, increments the session turn counter, resets the compact-requested flag so it can re-trigger next turn, and flushes the in-memory store to disk.
SYMBOLS:
- Flush() error
TYPE: flushInput { SessionID string, ContextWindow *contextWindow }
TYPE: contextWindow { UsedPercentage float64 }
DEPENDS: internal/session
PATTERNS: stop-hook, flush-to-disk

---

# hook/post.go

DOES: Handles the PostToolUse hook — extracts text content from Bash/Read/MCP/generic/nested tool response shapes (skipping binary or huge-line outputs and error responses), runs the compression pipeline, stores a tool-input cache entry, persists the store to disk after every call, and returns the compressed output either via updatedToolOutput (preserving the original response shape) or as additionalContext.
SYMBOLS:
- Post() error
- postWithStore(store *session.Store, inp postInput, w io.Writer, persist bool) error
- toolText(raw json.RawMessage) string
- collectText(v any) []string
- isProbablyBinaryOrHugeLine(s string) bool
- updatedToolOutput(raw json.RawMessage, replacement string) (any, bool)
- isError(raw json.RawMessage) bool
TYPE: postInput { SessionID string, ToolName string, ToolInput map[string]any, ToolResponse json.RawMessage }
DEPENDS: internal/compress, internal/config, internal/session
PATTERNS: post-hook, adapter
USE WHEN: Adding support for a new tool response JSON shape; debugging why a response is not being replaced vs. injected as additionalContext.

---

# hook/pre.go

DOES: Handles the PreToolUse hook — looks up the deterministic tool-input cache key in the session store and, on a hit, injects the stored compressed result as additionalContext (also prepending any auto-compact nudge if warranted); on a miss, still checks for a pending compact nudge and emits it without any disk write on the miss path.
SYMBOLS:
- Pre() error
- preWithStore(store *session.Store, inp preInput, w io.Writer) error
- buildCacheKey(toolName string, input map[string]any) string
TYPE: preInput { SessionID string, ToolName string, ToolInput map[string]any }
DEPENDS: internal/config, internal/session
PATTERNS: pre-hook, cache-lookup
USE WHEN: Changing how tool-input cache keys are constructed; debugging why a pre-hook hit is or is not firing.

---

# hook/replay.go

DOES: Reads a JSONL session log file and replays each entry through the compression pipeline with current settings using a fresh ephemeral store, accumulating per-entry savings and printing either a human-readable table or a structured JSON report for offline threshold tuning.
SYMBOLS:
- Replay(logPath string) error
- ReplayJSON(logPath string) error
- collectReplay(logPath string) (ReplayReport, error)
- newEphemeralStore() *session.Store
TYPE: ReplayEntry { ToolName string, ToolInput map[string]any, Content string }
TYPE: ReplayReport { Entries []ReplayResult, Total ReplayTotal }
TYPE: ReplayResult { Line int, ToolName string, Original int, Final int, Saved int, Strategies []string }
TYPE: ReplayTotal { Entries int, Original int, Final int, Saved int, Ratio float64 }
DEPENDS: internal/compress, internal/session
PATTERNS: replay, offline-analysis
USE WHEN: Tuning compression thresholds or validating a new strategy against a recorded session log.
