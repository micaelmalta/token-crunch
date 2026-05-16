---
room: session/store
see_also:
  - compress/core.md
  - hook/hooks.md
hot_paths:
  - session/store.go → Put, Get, Flush
  - session/diff.go → DiffRatio, UnifiedDiff
architectural_health: normal
security_tier: normal
---

# session/diff.go

DOES: Computes token-level similarity between two strings using an LCS-based DiffRatio score, and generates a compact unified +/- diff with context lines around changes via a full DP edit-script (falling back to a greedy O(m+n) algorithm for very large inputs above 500k DP cells).
SYMBOLS:
- DiffRatio(a, b string) float64
- UnifiedDiff(oldContent, newContent string) string
- NearMatchThreshold (const = 0.85)
- tokenise(s string) []string
- lcsLen(a, b []string) int
- shortestEditScript(a, b []string) []byte
- greedyEditScript(a, b []string) []byte
- min3(a, b, c int) int
PATTERNS: lcs-similarity, unified-diff, dp-edit-script
USE WHEN: Adjusting near-match similarity scoring; understanding how unified diff output is produced for the dedup path.

---

# session/store.go

DOES: Provides the content-addressed, turn-aware in-memory session store with SHA-256 keying (truncated to 16 bytes), strategy metadata on each entry, a separate tool-input result cache (GetToolCache/PutToolCache), PreToolUse hit/miss metrics, context-window percentage tracking, compact-request flag, LRU-style pruning when the JSON serialisation exceeds MaxSessionBytes, atomic JSON flush and load, a process-level singleton (Init/Global), per-strategy and per-tool stats reporting in both text and JSON formats, and session file cleanup by age.
SYMBOLS:
- NewEphemeral() *Store
- Init(sessionID string)
- Global() *Store
- Hash(content string) string
- (s *Store) Get(content string) *Entry
- (s *Store) GetByHash(h string) *Entry
- (s *Store) GetToolCache(h string) *Entry
- (s *Store) Put(toolName, content string, compressedSize int) string
- (s *Store) PutWithStrategies(toolName, content string, compressedSize int, strategies []string, storeRaw bool) string
- (s *Store) PutToolCache(cacheKey, toolName, content string, originalSize int) string
- (s *Store) PutToolCacheWithRaw(cacheKey, toolName, content string, originalSize int, storeRaw bool) string
- (s *Store) RecordPreCacheHit()
- (s *Store) RecordPreCacheMiss()
- (s *Store) Metrics() Metrics
- (s *Store) SetContextUsedPct(pct float64)
- (s *Store) ContextUsedPct() float64
- (s *Store) SetCompactRequested(v bool)
- (s *Store) CompactRequested() bool
- (s *Store) IncrementTurn()
- (s *Store) CurrentTurn() int
- (s *Store) AllEntries() []*Entry
- (s *Store) Flush() error
- Stats() error
- StatsJSON() error
- StatsText() error
- Clean(days int) error
TYPE: Store { mu sync.Mutex, entries map[string]*Entry, toolCache map[string]*Entry, metrics Metrics, turn int, path string, contextUsedPct float64, compactRequested bool }
TYPE: Entry { Hash string, Turn int, ToolName string, OriginalSize int, CompressedSize int, Strategies []string, StoredAt time.Time, Content string }
TYPE: Metrics { PreCacheHits int, PreCacheMisses int }
TYPE: StatsReport { Sessions []SessionReport, Strategies []BucketReport, Tools []BucketReport, TopOutputs []EntryReport, TopCache []EntryReport, Timeline []EntryReport, Total BucketReport, PreCacheHits int, PreCacheMisses int }
DEPENDS: internal/config
PATTERNS: content-addressable-cache, singleton, atomic-write, lru-eviction
USE WHEN: Adding new per-entry metadata; changing the persistence format; debugging cache misses; extending stats reporting.
