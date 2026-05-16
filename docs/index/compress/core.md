---
room: compress/core
see_also:
  - hook/hooks.md
  - session/store.md
hot_paths:
  - compress/pipeline.go → Run
  - compress/dedup.go → DedupWithThreshold
  - compress/structure.go → CollapseStructure
  - compress/relevance.go → trimByRelevance
architectural_health: normal
security_tier: normal
---

# compress/dedup.go

DOES: Applies differential deduplication against the session store — returns a compact exact-match reference when the content was seen before, or a near-match unified diff header when a prior entry scores above the similarity threshold.
SYMBOLS:
- Dedup(store *session.Store, toolName, content string) (string, bool)
- DedupWithThreshold(store *session.Store, toolName, content string, threshold float64) (string, bool)
- findNearMatch(store *session.Store, content string, threshold float64) *session.Entry
DEPENDS: internal/session
PATTERNS: content-addressable-cache, near-match-diff
USE WHEN: Debugging why repeated tool outputs are or are not being collapsed; adjusting near-match threshold behavior.

---

# compress/pipeline.go

DOES: Orchestrates the three-stage compression pipeline (dedup → structural collapse → relevance trimming), gates each stage on the Config.StrategyEnabled flag, prepends a token-savings header to modified output, records a per-call decision trace, and passes error signals through losslessly without modification.
SYMBOLS:
- Run(store *session.Store, toolName string, toolInput map[string]any, content string, tokenBudget int) Result
- RunWithConfig(store *session.Store, toolName string, toolInput map[string]any, content string, cfg config.Config) Result
- looksLikeErrorWithKeywords(content string, keywords []string) bool
- formatNum(n int) string
TYPE: Result { Output string, OrigSize int, FinalSize int, Strategies []string, WasModified bool, Trace []string }
DEPENDS: internal/config, internal/session
PATTERNS: pipeline-chain, lossless-passthrough
USE WHEN: Tracing which strategy fired for a given input; adding a new stage to the pipeline.

---

# compress/relevance.go

DOES: Scores each output line using TF-IDF against the tool's query terms extracted from tool_input, drops low-scoring lines when content exceeds the token budget, always preserving the first and last lines and any line containing a query term verbatim. Also provides EstimateTokens, the shared Claude-token-count approximation used across the package.
SYMBOLS:
- TrimByRelevance(content string, queryTerms []string, tokenBudget int) (string, bool)
- ExtractQueryTerms(toolInput map[string]any) []string
- EstimateTokens(s string) int
- computeIDF(lines []string) map[string]float64
- tfIDF(line string, querySet map[string]bool, idf map[string]float64) float64
- normaliseTerms(terms []string) map[string]bool
- containsAny(line string, terms map[string]bool) bool
- tokeniseWords(s string) []string
- technicalRatio(words []string) float64
- isTechnicalWord(w string) bool
- countPuncTokens(s string) int
DEPENDS: internal/config
PATTERNS: tf-idf-scoring, budget-trim
USE WHEN: Adding a new line-level filter strategy; debugging why content is over the token budget; changing the token estimation model.

---

# compress/structure.go

DOES: Detects output shape via a priority-ordered series of probes (JSON array/object, NDJSON, CSV, unified diff, diagnostics, coverage, package logs, markup, stack trace, file tree, test output, log stream, tabular with column-pruning, YAML, TOML) and applies a format-specific structural collapse to reduce repetitive or low-signal lines. Tabular collapse also applies a row-level clip against the token budget when the column-pruned table still exceeds it.
SYMBOLS:
- CollapseStructure(content string, toolArgs map[string]any) (string, bool)
- tryJSONArray(content string) (string, bool)
- tryJSONObject(content string) (string, bool)
- tryNDJSON(content string) (string, bool)
- tryCSV(content string) (string, bool)
- tryYAML(content string) (string, bool)
- tryTOML(content string) (string, bool)
- tryUnifiedDiff(content string) (string, bool)
- tryDiagnostics(content string) (string, bool)
- tryCoverage(content string) (string, bool)
- tryPackageLog(content string) (string, bool)
- tryMarkup(content string) (string, bool)
- tryStackTrace(content string) (string, bool)
- tryFileTree(content string) (string, bool)
- tryTestOutput(content string) (string, bool)
- tryLogStream(content string) (string, bool)
- tryTabularWithBudget(content string, toolArgs map[string]any, tokenBudget int) (string, bool)
- leadingSpaces(s string) int
- argsToString(args map[string]any) string
- nonEmptyLines(content string) []string
- sortStrings(values []string)
DEPENDS: internal/config
PATTERNS: strategy-pattern, shape-detection
USE WHEN: Adding support for a new output format; debugging why a specific tool output is not being collapsed; adjusting collapse thresholds per shape.
