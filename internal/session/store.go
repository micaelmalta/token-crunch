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

	"github.com/micaelmalta/token-crunch/internal/config"
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
	Name       string    `json:"name"`
	Entries    int       `json:"entries"`
	ToolCache  int       `json:"tool_cache"`
	Original   int       `json:"original"`
	Compressed int       `json:"compressed"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	LastActive time.Time `json:"last_active,omitempty"`
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

	// measure longest session name for dynamic column width
	nameW := len("Session")
	for _, ss := range report.Sessions {
		if len(ss.Name) > nameW {
			nameW = len(ss.Name)
		}
	}
	const dtW = 17 // "2006-01-02 15:04"
	rowW := nameW + 2 + dtW + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 6
	hdrFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%8s  %%8s  %%8s  %%6s\n", nameW, dtW)
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%8s  %%8s  %%8s  %%5.1f%%%%\n", nameW, dtW)
	fmt.Printf(hdrFmt, "Session", "Started", "Orig", "Comp", "Saved", "Ratio")
	fmt.Println(strings.Repeat("-", rowW))
	for _, ss := range report.Sessions {
		dt := ""
		if !ss.StartedAt.IsZero() {
			dt = ss.StartedAt.Format("2006-01-02 15:04")
		}
		saved := ss.Original - ss.Compressed
		ratio := 0.0
		if ss.Original > 0 {
			ratio = float64(saved) / float64(ss.Original) * 100
		}
		fmt.Printf(rowFmt, ss.Name, dt, humanBytes(ss.Original), humanBytes(ss.Compressed), humanBytes(saved), ratio)
	}
	fmt.Println(strings.Repeat("-", rowW))
	// TOTAL row (no datetime)
	saved := report.Total.Original - report.Total.Compressed
	ratio := 0.0
	if report.Total.Original > 0 {
		ratio = float64(saved) / float64(report.Total.Original) * 100
	}
	fmt.Printf(rowFmt, "TOTAL", "", humanBytes(report.Total.Original), humanBytes(report.Total.Compressed), humanBytes(saved), ratio)

	if len(report.Strategies) > 0 {
		fmt.Println("\nBy strategy")
		printBucketTable("Strategy", report.Strategies)
	}
	if len(report.Tools) > 0 {
		fmt.Println("\nBy tool")
		printBucketTable("Tool", report.Tools)
	}
	if len(report.TopOutputs) > 0 {
		fmt.Println("\nTop saved outputs")
		printEntryTable(report.TopOutputs, "Saved", func(e EntryReport) string { return humanBytes(e.Saved) })
	}
	if len(report.TopCache) > 0 {
		fmt.Println("\nTop pre-cache entries")
		printEntryTable(report.TopCache, "Orig", func(e EntryReport) string { return humanBytes(e.Original) })
	}
	if len(report.Timeline) > 0 {
		fmt.Println("\nTimeline (most recent)")
		end := len(report.Timeline)
		start := end - 10
		if start < 0 {
			start = 0
		}
		printTimelineTable(report.Timeline[start:end])
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
		var firstSeen, lastSeen time.Time
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
			if !e.StoredAt.IsZero() {
				if firstSeen.IsZero() || e.StoredAt.Before(firstSeen) {
					firstSeen = e.StoredAt
				}
				if e.StoredAt.After(lastSeen) {
					lastSeen = e.StoredAt
				}
			}
		}
		for _, e := range df.ToolCache {
			report.TopCache = append(report.TopCache, entryReport(de.Name(), e))
		}
		// fall back to file mtime when no entry timestamps exist
		if firstSeen.IsZero() {
			if fi, err := de.Info(); err == nil {
				firstSeen = fi.ModTime()
				lastSeen = fi.ModTime()
			}
		}
		report.Sessions = append(report.Sessions, SessionReport{
			Name:       de.Name(),
			Entries:    len(df.Entries),
			ToolCache:  len(df.ToolCache),
			Original:   orig,
			Compressed: comp,
			StartedAt:  firstSeen,
			LastActive: lastSeen,
		})
		report.Total.Original += orig
		report.Total.Compressed += comp
		report.Total.Entries += len(df.Entries)
		report.PreCacheHits += df.Metrics.PreCacheHits
		report.PreCacheMisses += df.Metrics.PreCacheMisses
	}

	report.Total.Name = "TOTAL"
	sort.Slice(report.Sessions, func(i, j int) bool {
		return report.Sessions[i].StartedAt.After(report.Sessions[j].StartedAt)
	})
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

func printBucketTable(header string, buckets []BucketReport) {
	nameW := len(header)
	for _, b := range buckets {
		if len(b.Name) > nameW {
			nameW = len(b.Name)
		}
	}
	rowW := nameW + 2 + 8 + 2 + 8 + 2 + 8 + 2 + 6
	hdr := fmt.Sprintf("%%-%ds  %%8s  %%8s  %%8s  %%6s\n", nameW)
	row := fmt.Sprintf("%%-%ds  %%8s  %%8s  %%8s  %%5.1f%%%%\n", nameW)
	fmt.Printf(hdr, header, "Orig", "Comp", "Saved", "Ratio")
	fmt.Println(strings.Repeat("-", rowW))
	for _, b := range buckets {
		saved := b.Original - b.Compressed
		ratio := 0.0
		if b.Original > 0 {
			ratio = float64(saved) / float64(b.Original) * 100
		}
		fmt.Printf(row, b.Name, humanBytes(b.Original), humanBytes(b.Compressed), humanBytes(saved), ratio)
	}
}

func printEntryTable(entries []EntryReport, valueHeader string, value func(EntryReport) string) {
	toolW := len("Tool")
	hashW := len("Hash")
	for _, e := range entries {
		if len(e.ToolName) > toolW {
			toolW = len(e.ToolName)
		}
		if len(e.Hash) > hashW {
			hashW = len(e.Hash)
		}
	}
	rowW := toolW + 2 + hashW + 2 + 8 + 2 + 5
	hdr := fmt.Sprintf("%%-%ds  %%-%ds  %%8s  %%5s\n", toolW, hashW)
	row := fmt.Sprintf("%%-%ds  %%-%ds  %%8s  %%5d\n", toolW, hashW)
	fmt.Printf(hdr, "Tool", "Hash", valueHeader, "Turn")
	fmt.Println(strings.Repeat("-", rowW))
	for _, e := range entries {
		fmt.Printf(row, e.ToolName, e.Hash, value(e), e.Turn)
	}
}

func printTimelineTable(entries []EntryReport) {
	toolW := len("Tool")
	hashW := len("Hash")
	for _, e := range entries {
		if len(e.ToolName) > toolW {
			toolW = len(e.ToolName)
		}
		if len(e.Hash) > hashW {
			hashW = len(e.Hash)
		}
	}
	const storedW = 25
	rowW := toolW + 2 + hashW + 2 + storedW + 2 + 8
	hdr := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%8s\n", toolW, hashW, storedW)
	row := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%8s\n", toolW, hashW, storedW)
	fmt.Printf(hdr, "Tool", "Hash", "Stored", "Saved")
	fmt.Println(strings.Repeat("-", rowW))
	for _, e := range entries {
		fmt.Printf(row, e.ToolName, e.Hash, e.StoredAt, humanBytes(e.Saved))
	}
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
