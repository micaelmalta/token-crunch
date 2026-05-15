---
room: entrypoints
see_also:
  - hook/hooks.md
  - session/store.md
  - compress/core.md
architectural_health: normal
security_tier: normal
---

# cmd/benchmark/main.go

DOES: CLI benchmark harness that runs a fixed set of realistic fixture scenarios through the compression pipeline and prints a per-scenario savings table; used for tuning and regression testing.
SYMBOLS:
- main()
- runCase(c case_) (origTokens, finalTokens int, strategy string)
- repeatedFileRead() case_
- repeatedGitStatus() case_
- jsonArrayOutput() case_
- stackTraceOutput() case_
- fileTreeOutput() case_
- testOutputPassFail() case_
- logStreamDedup() case_
- relevanceTrimming() case_
- combinedSession() case_
- repeat(s string, n int) []string
- estimateTokens(s string) int
- humanTokens(n int) string
- Types: case_ { name, toolName, toolInput, turns }
DEPENDS: internal/compress, internal/session
PATTERNS: benchmark-harness, fixture-driven

---

# cmd/token-crunch/main.go

DOES: Main CLI dispatcher that routes subcommands (pre, post, flush, install [--local], uninstall [--local], stats, clean, explain, replay, config validate/show, version) to their handler packages, including JSON modes for stats and replay.
SYMBOLS:
- main()
- usage()
DEPENDS: internal/hook, internal/install, internal/session, internal/config

---

# internal/install/install.go

DOES: Merges and removes token-crunch hook entries in ~/.claude/settings.json (global) or .claude/settings.local.json (local, pass local=true) non-destructively, using atomic file writes to avoid corruption.
SYMBOLS:
- Install(local bool) error
- Uninstall(local bool) error
- mergeHook(existing []hookEntry, entry hookEntry) []hookEntry
- removeHook(existing []hookEntry, cmd string) []hookEntry
- loadOrEmpty(path string) []byte
- writeAtomic(path string, top map[string]json.RawMessage) error
- claudeSettingsPath(local bool) string
- Types: hookCommand { Type, Command }, hookEntry { Matcher, Hooks }, hooksSection { PreToolUse, PostToolUse, Stop }
PATTERNS: atomic-write, idempotent-merge
