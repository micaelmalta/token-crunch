package session

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmalta/token-crunch/internal/config"
)

// Entry records a single cached tool output in the session store.
type Entry struct {
	Hash           string    `json:"hash"`
	Turn           int       `json:"turn"`
	ToolName       string    `json:"tool_name"`
	OriginalSize   int       `json:"original_size"`
	CompressedSize int       `json:"compressed_size"`
	Strategies     []string  `json:"strategies,omitempty"`
	StoredAt       time.Time `json:"stored_at"`
	Content        string    `json:"content"`
}

type Metrics struct {
	PreCacheHits   int `json:"pre_cache_hits"`
	PreCacheMisses int `json:"pre_cache_misses"`
}

// Store is the in-memory, content-addressed session store.
type Store struct {
	mu        sync.Mutex
	entries   map[string]*Entry
	toolCache map[string]*Entry
	metrics   Metrics
	turn      int
	path      string
}

var global *Store
var globalOnce sync.Once

// NewEphemeral creates an in-memory store with no disk backing (used for replay).
func NewEphemeral() *Store {
	return &Store{
		entries:   make(map[string]*Entry),
		toolCache: make(map[string]*Entry),
	}
}

// Init initialises the global store with an explicit session ID.
// Must be called before Global() if you have a session ID from the hook payload.
// Safe to call multiple times — only the first call takes effect.
func Init(sessionID string) {
	globalOnce.Do(func() {
		global = &Store{
			entries:   make(map[string]*Entry),
			toolCache: make(map[string]*Entry),
		}
		global.path = sessionFilePath(sessionID)
		_ = global.load()
	})
}

// Global returns the process-level store. Call Init first if you have a session ID.
func Global() *Store {
	globalOnce.Do(func() {
		global = &Store{
			entries:   make(map[string]*Entry),
			toolCache: make(map[string]*Entry),
		}
		global.path = sessionFilePath(fallbackSessionID())
		_ = global.load()
	})
	return global
}

func fallbackSessionID() string {
	if id := os.Getenv("CLAUDE_SESSION_ID"); id != "" {
		return id
	}
	return "default"
}

func sessionFilePath(sessionID string) string {
	return filepath.Join(storeDir(), "session-"+sessionID+".json")
}

func storeDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "token-crunch")
}

// Hash returns a stable key for content (normalised: trimmed whitespace).
func Hash(content string) string {
	normalised := strings.TrimSpace(content)
	sum := sha256.Sum256([]byte(normalised))
	return fmt.Sprintf("%x", sum[:16])
}

// Get returns an Entry if the exact content hash is present, nil otherwise.
func (s *Store) Get(content string) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := Hash(content)
	return s.entries[h]
}

// GetByHash returns an Entry by pre-computed hash.
func (s *Store) GetByHash(h string) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entries[h]
}

// GetToolCache returns a cached tool result by tool-input cache hash.
func (s *Store) GetToolCache(h string) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.toolCache[h]
}

// Put stores content and returns its hash.
func (s *Store) Put(toolName, content string, compressedSize int) string {
	return s.PutWithStrategies(toolName, content, compressedSize, nil, true)
}

// PutWithStrategies stores content with strategy metadata and returns its hash.
func (s *Store) PutWithStrategies(toolName, content string, compressedSize int, strategies []string, storeRaw bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]*Entry)
	}
	h := Hash(content)
	storedContent := content
	if !storeRaw {
		storedContent = ""
	}
	s.entries[h] = &Entry{
		Hash:           h,
		Turn:           s.turn,
		ToolName:       toolName,
		OriginalSize:   len(content),
		CompressedSize: compressedSize,
		Strategies:     append([]string(nil), strategies...),
		StoredAt:       time.Now(),
		Content:        storedContent,
	}
	return h
}

// PutToolCache stores the result associated with a tool-input cache hash.
func (s *Store) PutToolCache(cacheKey, toolName, content string, originalSize int) string {
	return s.PutToolCacheWithRaw(cacheKey, toolName, content, originalSize, true)
}

// PutToolCacheWithRaw stores the result associated with a tool-input cache hash.
func (s *Store) PutToolCacheWithRaw(cacheKey, toolName, content string, originalSize int, storeRaw bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.toolCache == nil {
		s.toolCache = make(map[string]*Entry)
	}
	h := Hash(cacheKey)
	storedContent := content
	if !storeRaw {
		storedContent = ""
	}
	s.toolCache[h] = &Entry{
		Hash:           h,
		Turn:           s.turn,
		ToolName:       toolName,
		OriginalSize:   originalSize,
		CompressedSize: len(content),
		StoredAt:       time.Now(),
		Content:        storedContent,
	}
	return h
}

func (s *Store) RecordPreCacheHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.PreCacheHits++
}

func (s *Store) RecordPreCacheMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.PreCacheMisses++
}

func (s *Store) Metrics() Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metrics
}

// IncrementTurn bumps the session turn counter.
func (s *Store) IncrementTurn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turn++
}

// CurrentTurn returns the current turn number.
func (s *Store) CurrentTurn() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.turn
}

// AllEntries returns a snapshot of all entries, sorted by turn.
func (s *Store) AllEntries() []*Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := make([]*Entry, 0, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Turn < entries[j].Turn
	})
	return entries
}

type diskFormat struct {
	Turn      int               `json:"turn"`
	Entries   map[string]*Entry `json:"entries"`
	ToolCache map[string]*Entry `json:"tool_cache,omitempty"`
	Metrics   Metrics           `json:"metrics,omitempty"`
}

// Flush atomically writes the store to its session file.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	df := diskFormat{Turn: s.turn, Entries: s.entries, ToolCache: s.toolCache, Metrics: s.metrics}
	data, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return err
	}
	cfg := config.Load()
	if cfg.MaxSessionBytes > 0 && int64(len(data)) > cfg.MaxSessionBytes {
		pruneEntries(s.entries, cfg.MaxSessionBytes/2)
		df.Entries = s.entries
		data, err = json.MarshalIndent(df, "", "  ")
		if err != nil {
			return err
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil // not found is fine — fresh session
	}
	var df diskFormat
	if err := json.Unmarshal(data, &df); err != nil {
		return err
	}
	s.turn = df.Turn
	if df.Entries != nil {
		s.entries = df.Entries
	}
	if s.entries == nil {
		s.entries = make(map[string]*Entry)
	}
	if df.ToolCache != nil {
		s.toolCache = df.ToolCache
	}
	if s.toolCache == nil {
		s.toolCache = make(map[string]*Entry)
	}
	s.metrics = df.Metrics
	return nil
}

func pruneEntries(entries map[string]*Entry, targetBytes int64) {
	if targetBytes <= 0 || len(entries) == 0 {
		return
	}
	type item struct {
		hash string
		e    *Entry
	}
	items := make([]item, 0, len(entries))
	var total int64
	for h, e := range entries {
		items = append(items, item{hash: h, e: e})
		total += int64(len(e.Content))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].e.StoredAt.Before(items[j].e.StoredAt)
	})
	for _, it := range items {
		if total <= targetBytes {
			return
		}
		total -= int64(len(it.e.Content))
		delete(entries, it.hash)
	}
}

// Stats prints cumulative savings across all session files to stdout.
func Stats() error {
	return StatsText()
}

type StatsReport struct {
	Sessions       []SessionReport `json:"sessions"`
	Strategies     []BucketReport  `json:"strategies"`
	Tools          []BucketReport  `json:"tools"`
	TopOutputs     []EntryReport   `json:"top_outputs"`
	TopCache       []EntryReport   `json:"top_cache"`
	Timeline       []EntryReport   `json:"timeline"`
	Total          BucketReport    `json:"total"`
	PreCacheHits   int             `json:"pre_cache_hits"`
	PreCacheMisses int             `json:"pre_cache_misses"`
}

type SessionReport struct {
	Name       string `json:"name"`
	Entries    int    `json:"entries"`
	ToolCache  int    `json:"tool_cache"`
	Original   int    `json:"original"`
	Compressed int    `json:"compressed"`
}

type BucketReport struct {
	Name       string `json:"name"`
	Entries    int    `json:"entries"`
	Original   int    `json:"original"`
	Compressed int    `json:"compressed"`
}

type EntryReport struct {
	Session    string   `json:"session"`
	Hash       string   `json:"hash"`
	Turn       int      `json:"turn"`
	ToolName   string   `json:"tool_name"`
	Original   int      `json:"original"`
	Compressed int      `json:"compressed"`
	Saved      int      `json:"saved"`
	Strategies []string `json:"strategies,omitempty"`
	StoredAt   string   `json:"stored_at,omitempty"`
}

func StatsJSON() error {
	report, err := collectStats()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func StatsText() error {
	report, err := collectStats()
	if err != nil {
		return err
	}
	if len(report.Sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	totalEntries := 0
	for _, s := range report.Sessions {
		totalEntries += s.Entries
	}
	fmt.Printf("token-crunch stats — %d session(s), %d cached outputs\n", len(report.Sessions), totalEntries)
	fmt.Printf("PreToolUse cache — %d hit(s), %d miss(es)\n\n", report.PreCacheHits, report.PreCacheMisses)
	fmt.Printf("%-40s  %8s  %8s  %8s  %6s\n", "Session", "Orig", "Comp", "Saved", "Ratio")
	fmt.Printf("%s\n", strings.Repeat("-", 80))
	for _, ss := range report.Sessions {
		printBucket(ss.Name, ss.Original, ss.Compressed)
	}
	fmt.Printf("%s\n", strings.Repeat("-", 80))
	printBucket("TOTAL", report.Total.Original, report.Total.Compressed)

	if len(report.Strategies) > 0 {
		fmt.Println("\nBy strategy")
		fmt.Printf("%-20s  %8s  %8s  %8s  %6s\n", "Strategy", "Orig", "Comp", "Saved", "Ratio")
		for _, b := range report.Strategies {
			printBucket(b.Name, b.Original, b.Compressed)
		}
	}
	if len(report.Tools) > 0 {
		fmt.Println("\nBy tool")
		fmt.Printf("%-20s  %8s  %8s  %8s  %6s\n", "Tool", "Orig", "Comp", "Saved", "Ratio")
		for _, b := range report.Tools {
			printBucket(b.Name, b.Original, b.Compressed)
		}
	}
	if len(report.TopOutputs) > 0 {
		fmt.Println("\nTop saved outputs")
		fmt.Printf("%-20s  %-16s  %8s  %8s\n", "Tool", "Hash", "Saved", "Turn")
		for _, e := range report.TopOutputs {
			fmt.Printf("%-20s  %-16s  %8s  %8d\n", e.ToolName, e.Hash, humanBytes(e.Saved), e.Turn)
		}
	}
	if len(report.TopCache) > 0 {
		fmt.Println("\nTop pre-cache entries")
		fmt.Printf("%-20s  %-16s  %8s  %8s\n", "Tool", "Hash", "Orig", "Turn")
		for _, e := range report.TopCache {
			fmt.Printf("%-20s  %-16s  %8s  %8d\n", e.ToolName, e.Hash, humanBytes(e.Original), e.Turn)
		}
	}
	if len(report.Timeline) > 0 {
		fmt.Println("\nTimeline (most recent)")
		fmt.Printf("%-20s  %-16s  %-25s  %8s\n", "Tool", "Hash", "Stored", "Saved")
		end := len(report.Timeline)
		start := end - 10
		if start < 0 {
			start = 0
		}
		for _, e := range report.Timeline[start:end] {
			fmt.Printf("%-20s  %-16s  %-25s  %8s\n", e.ToolName, e.Hash, e.StoredAt, humanBytes(e.Saved))
		}
	}
	return nil
}

func collectStats() (StatsReport, error) {
	dir := storeDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return StatsReport{}, nil
		}
		return StatsReport{}, err
	}

	report := StatsReport{}
	byStrategy := map[string]*BucketReport{}
	byTool := map[string]*BucketReport{}

	for _, de := range entries {
		if !strings.HasSuffix(de.Name(), ".json") || strings.HasSuffix(de.Name(), ".tmp") {
			continue
		}
		path := filepath.Join(dir, de.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var df diskFormat
		if err := json.Unmarshal(data, &df); err != nil {
			continue
		}
		var orig, comp int
		for _, e := range df.Entries {
			orig += e.OriginalSize
			comp += e.CompressedSize
			addBucket(byTool, e.ToolName, e)
			if len(e.Strategies) == 0 {
				addBucket(byStrategy, "none", e)
			} else {
				for _, strat := range e.Strategies {
					addBucket(byStrategy, strat, e)
				}
			}
			report.TopOutputs = append(report.TopOutputs, entryReport(de.Name(), e))
			report.Timeline = append(report.Timeline, entryReport(de.Name(), e))
		}
		for _, e := range df.ToolCache {
			report.TopCache = append(report.TopCache, entryReport(de.Name(), e))
		}
		report.Sessions = append(report.Sessions, SessionReport{
			Name:       de.Name(),
			Entries:    len(df.Entries),
			ToolCache:  len(df.ToolCache),
			Original:   orig,
			Compressed: comp,
		})
		report.Total.Original += orig
		report.Total.Compressed += comp
		report.Total.Entries += len(df.Entries)
		report.PreCacheHits += df.Metrics.PreCacheHits
		report.PreCacheMisses += df.Metrics.PreCacheMisses
	}

	report.Total.Name = "TOTAL"
	report.Strategies = bucketSlice(byStrategy)
	report.Tools = bucketSlice(byTool)
	sortEntryReportsBySaved(report.TopOutputs)
	sortEntryReportsBySaved(report.TopCache)
	sort.Slice(report.Timeline, func(i, j int) bool {
		return report.Timeline[i].StoredAt < report.Timeline[j].StoredAt
	})
	report.TopOutputs = limitEntryReports(report.TopOutputs, 10)
	report.TopCache = limitEntryReports(report.TopCache, 10)
	report.Timeline = limitEntryReports(report.Timeline, 50)
	return report, nil
}

func addBucket(m map[string]*BucketReport, name string, e *Entry) {
	if name == "" {
		name = "unknown"
	}
	b := m[name]
	if b == nil {
		b = &BucketReport{Name: name}
		m[name] = b
	}
	b.Entries++
	b.Original += e.OriginalSize
	b.Compressed += e.CompressedSize
}

func bucketSlice(m map[string]*BucketReport) []BucketReport {
	out := make([]BucketReport, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Original-out[i].Compressed > out[j].Original-out[j].Compressed
	})
	return out
}

func printBucket(name string, orig, comp int) {
	saved := orig - comp
	ratio := 0.0
	if orig > 0 {
		ratio = float64(saved) / float64(orig) * 100
	}
	fmt.Printf("%-40s  %8s  %8s  %8s  %5.1f%%\n",
		name,
		humanBytes(orig),
		humanBytes(comp),
		humanBytes(saved),
		ratio,
	)
}

func entryReport(session string, e *Entry) EntryReport {
	storedAt := ""
	if !e.StoredAt.IsZero() {
		storedAt = e.StoredAt.Format(time.RFC3339)
	}
	return EntryReport{
		Session:    session,
		Hash:       e.Hash,
		Turn:       e.Turn,
		ToolName:   e.ToolName,
		Original:   e.OriginalSize,
		Compressed: e.CompressedSize,
		Saved:      e.OriginalSize - e.CompressedSize,
		Strategies: append([]string(nil), e.Strategies...),
		StoredAt:   storedAt,
	}
}

func sortEntryReportsBySaved(entries []EntryReport) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Saved > entries[j].Saved
	})
}

func limitEntryReports(entries []EntryReport, n int) []EntryReport {
	if len(entries) <= n {
		return entries
	}
	return entries[:n]
}

func Clean(days int) error {
	dir := storeDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, de := range entries {
		if !strings.HasSuffix(de.Name(), ".json") || strings.HasSuffix(de.Name(), ".tmp") {
			continue
		}
		path := filepath.Join(dir, de.Name())
		info, err := de.Info()
		if err != nil {
			continue
		}
		if days <= 0 || info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func humanBytes(b int) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
