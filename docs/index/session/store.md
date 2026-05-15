---
room: session/store
see_also:
  - compress/core.md
  - hook/hooks.md
hot_paths:
  - session/store.go → Put, Get, Flush
  - session/diff.go → DiffRatio
architectural_health: normal
security_tier: normal
---

# session/diff.go

DOES: Computes token-level similarity between two strings (LCS-based DiffRatio) and generates a compact +/- unified diff used by the near-match dedup path.
SYMBOLS:
- DiffRatio(a, b string) float64
- UnifiedDiff(oldContent, newContent string) string
- NearMatchThreshold float64 (const = 0.85)
- tokenise(s string) []string
- lcsLen(a, b []string) int
- shortestEditScript(a, b []string) []byte
- min3(a, b, c int) int
PATTERNS: lcs-similarity, unified-diff, dp-edit-script

---

# session/store.go

DOES: Provides the content-addressed, turn-aware in-memory session store with SHA-256 keying, strategy metadata, PreToolUse cache metrics, a separate tool-input result cache, a singleton global initialised from a session ID, atomic JSON flush/load, top-output/cache/timeline stats reporting (all three sections printed in text mode), and cleanup for cross-process persistence.
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
- (s *Store) IncrementTurn()
- (s *Store) CurrentTurn() int
- (s *Store) AllEntries() []*Entry
- (s *Store) Flush() error
- Stats() error
- StatsJSON() error
- Clean(days int) error
- Types: Store { mu, entries, toolCache, metrics, turn, path }, Entry { Hash, Turn, ToolName, OriginalSize, CompressedSize, Strategies, StoredAt, Content }, Metrics { PreCacheHits, PreCacheMisses }, StatsReport, EntryReport
PATTERNS: content-addressable-cache, singleton, atomic-write
USE WHEN: Adding new per-entry metadata; changing the persistence format; debugging cache misses.
