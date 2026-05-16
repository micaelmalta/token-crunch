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

DOES: CLI benchmark harness that runs nine realistic fixture scenarios (repeated file reads, git status, JSON arrays, stack traces, file trees, test output, log streams, relevance trimming, combined sessions) through the compression pipeline and prints a per-scenario savings table with original/final token counts, ratio, and strategy labels.
SYMBOLS:
- main()
- runCase(c case_) (origTokens int, finalTokens int, strategy string)
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
TYPE: case_ { name string, toolName string, toolInput map[string]any, turns []string }
DEPENDS: internal/compress, internal/session
PATTERNS: benchmark-harness, fixture-driven

---

# cmd/token-crunch/main.go

DOES: Main CLI dispatcher that routes subcommands to their handler packages: pre/post/flush invoke hook handlers; install/uninstall write or remove hooks in settings files; stats/clean operate on the session store; explain/replay run offline diagnostics; config validate/show inspect the runtime configuration; version prints the build version.
SYMBOLS:
- main()
- usage()
DEPENDS: internal/hook, internal/install, internal/session, internal/config
PATTERNS: cli-dispatcher

---

# internal/config/config.go

DOES: Loads runtime configuration from environment variables (TOKEN_CRUNCH_*) with optional override from a JSON file specified by TOKEN_CRUNCH_CONFIG; environment variables always take precedence over file values. Exposes StrategyEnabled and Denies helpers used throughout the pipeline.
SYMBOLS:
- Load() Config
- (c Config) StrategyEnabled(name string) bool
- (c Config) Denies(s string) bool
- applyConfigFile(cfg Config, path string) Config
- applyEnvOverrides(cfg Config) Config
- envStrategies() map[string]bool
- envList(name string, def []string) []string
- envInt(name string, def int) int
- envFloat(name string, def float64) float64
- envString(name string, def string) string
- envBool(name string, def bool) bool
TYPE: Config { TokenBudget int, NearMatch float64, MinRelevanceLines int, ErrorKeywords []string, Strategies map[string]bool, MaxSessionBytes int64, RetentionDays int, StoreRaw bool, Debug bool, Denylist []string, CompactThreshold float64, CompactMessage string }
PATTERNS: env-driven-config, file-override
USE WHEN: Adding a new tuneable parameter; understanding how env vars interact with the JSON config file.

---

# internal/install/install.go

DOES: Merges token-crunch PreToolUse/PostToolUse/Stop hook entries into ~/.claude/settings.json (global) or .claude/settings.local.json (local) non-destructively, preserving all existing fields via a generic map parse, and atomically writes the result; Uninstall removes those entries by command string match.
SYMBOLS:
- Install(local bool) error
- Uninstall(local bool) error
- claudeSettingsPath(local bool) string
- mergeHook(existing []hookEntry, entry hookEntry) []hookEntry
- removeHook(existing []hookEntry, cmd string) []hookEntry
- loadOrEmpty(path string) []byte
- writeAtomic(path string, top map[string]json.RawMessage) error
TYPE: hookCommand { Type string, Command string }
TYPE: hookEntry { Matcher string, Hooks []hookCommand }
TYPE: hooksSection { PreToolUse []hookEntry, PostToolUse []hookEntry, Stop []hookEntry }
PATTERNS: atomic-write, idempotent-merge
USE WHEN: Changing the hook command strings; adding a new hook event type to the installer.
