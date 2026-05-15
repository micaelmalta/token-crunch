---
room: compress/core
see_also:
  - hook/hooks.md
  - session/store.md
hot_paths:
  - compress/pipeline.go → Run
  - compress/dedup.go → Dedup
architectural_health: normal
security_tier: normal
---

# compress/dedup.go

DOES: Applies differential deduplication against the session store, returning a compact exact-match reference or a near-match unified diff when the output is mostly unchanged from a prior turn.
SYMBOLS:
- Dedup(store *session.Store, toolName, content string) (string, bool)
- DedupWithThreshold(store *session.Store, toolName, content string, threshold float64) (string, bool)
- findNearMatch(store *session.Store, content string, threshold float64) *session.Entry
DEPENDS: internal/session
PATTERNS: content-addressable-cache, near-match-diff

---

# compress/pipeline.go

DOES: Orchestrates the configurable three-stage compression pipeline (dedup → structural collapse → relevance trimming), wraps the result with a savings header, records decision trace metadata, and passes error signals through losslessly.
SYMBOLS:
- Run(store *session.Store, toolName string, toolInput map[string]any, content string, tokenBudget int) Result
- RunWithConfig(store *session.Store, toolName string, toolInput map[string]any, content string, cfg config.Config) Result
- looksLikeError(content string) bool
- looksLikeErrorWithKeywords(content string, keywords []string) bool
- formatNum(n int) string
- Types: Result { Output, OrigSize, FinalSize, Strategies, WasModified, Trace }
DEPENDS: internal/config, internal/session
PATTERNS: pipeline-chain, lossless-passthrough

---

# compress/relevance.go

DOES: Scores each output line by TF-IDF against the tool's query terms and discards low-scoring lines when content exceeds the token budget, always preserving the first and last lines and any verbatim query matches.
SYMBOLS:
- TrimByRelevance(content string, queryTerms []string, tokenBudget int) (string, bool)
- ExtractQueryTerms(toolInput map[string]any) []string
- computeIDF(lines []string) map[string]float64
- tfIDF(line string, querySet map[string]bool, idf map[string]float64) float64
- normaliseTerms(terms []string) map[string]bool
- containsAny(line string, terms map[string]bool) bool
- tokeniseWords(s string) []string
- estimateTokens(s string) int
- indent(line string) int
- itoa(n int) string
PATTERNS: tf-idf-scoring, budget-trim
USE WHEN: Adding a new line-level filter strategy; debugging why content is over the token budget.

---

# compress/structure.go

DOES: Detects output shape (JSON object/array, NDJSON, CSV, YAML/TOML-like configs, unified diff, diagnostics, coverage, package logs, markup, stack trace, file tree, test output, log stream, tabular) and applies a format-specific structural collapse to reduce repetitive or low-signal lines.
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
- tryTabular(content string, toolArgs map[string]any) (string, bool)
- leadingSpaces(s string) int
- argsToString(args map[string]any) string
PATTERNS: strategy-pattern, shape-detection
USE WHEN: Adding support for a new output format (e.g. CSV, XML); debugging why a specific tool output is not being collapsed.
