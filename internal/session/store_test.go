package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHash_deterministic(t *testing.T) {
	h1 := Hash("hello world")
	h2 := Hash("hello world")
	if h1 != h2 {
		t.Fatal("same input must produce same hash")
	}
}

func TestHash_normalisesWhitespace(t *testing.T) {
	if Hash("  hello  ") != Hash("hello") {
		t.Fatal("hash must normalise surrounding whitespace")
	}
}

func TestHash_distinct(t *testing.T) {
	if Hash("foo") == Hash("bar") {
		t.Fatal("different inputs must not collide")
	}
}

func TestStore_putAndGet(t *testing.T) {
	s := NewEphemeral()
	content := "some tool output"
	s.Put("Bash", content, len(content))

	e := s.Get(content)
	if e == nil {
		t.Fatal("Get returned nil after Put")
	}
	if e.ToolName != "Bash" || e.Content != content {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestStore_putWithoutRawContent(t *testing.T) {
	s := NewEphemeral()
	content := "secret output"
	h := s.PutWithStrategies("Bash", content, 4, []string{"structure"}, false)

	e := s.GetByHash(h)
	if e == nil {
		t.Fatal("entry missing")
	}
	if e.Content != "" {
		t.Fatalf("raw content should not be stored, got %q", e.Content)
	}
	if len(e.Strategies) != 1 || e.Strategies[0] != "structure" {
		t.Fatalf("strategies not stored: %+v", e.Strategies)
	}
}

func TestStore_getMiss(t *testing.T) {
	s := NewEphemeral()
	if e := s.Get("not stored"); e != nil {
		t.Fatal("expected nil for unknown content")
	}
}

func TestStore_getByHash(t *testing.T) {
	s := NewEphemeral()
	content := "output"
	s.Put("Read", content, 6)
	h := Hash(content)
	if e := s.GetByHash(h); e == nil {
		t.Fatal("GetByHash returned nil for known hash")
	}
}

func TestStore_toolCachePutAndGet(t *testing.T) {
	s := NewEphemeral()
	cacheKey := `{"tool":"Read","input":{"file_path":"main.go"}}`
	content := "cached file content"
	h := s.PutToolCache(cacheKey, "Read", content, len(content))

	e := s.GetToolCache(h)
	if e == nil {
		t.Fatal("GetToolCache returned nil after PutToolCache")
	}
	if e.ToolName != "Read" || e.Content != content {
		t.Fatalf("unexpected tool cache entry: %+v", e)
	}
}

func TestStore_preCacheMetrics(t *testing.T) {
	s := NewEphemeral()
	s.RecordPreCacheHit()
	s.RecordPreCacheMiss()
	s.RecordPreCacheMiss()

	m := s.Metrics()
	if m.PreCacheHits != 1 || m.PreCacheMisses != 2 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}

func TestStore_incrementTurn(t *testing.T) {
	s := NewEphemeral()
	if s.CurrentTurn() != 0 {
		t.Fatal("initial turn must be 0")
	}
	s.IncrementTurn()
	s.IncrementTurn()
	if s.CurrentTurn() != 2 {
		t.Fatalf("want turn 2, got %d", s.CurrentTurn())
	}
}

func TestStore_allEntriesSortedByTurn(t *testing.T) {
	s := NewEphemeral()
	s.Put("Bash", "first", 5)
	s.IncrementTurn()
	s.Put("Read", "second", 6)
	s.IncrementTurn()
	s.Put("Bash", "third", 5)

	entries := s.AllEntries()
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Turn < entries[i-1].Turn {
			t.Fatal("entries not sorted by turn")
		}
	}
}

func TestStore_flushAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-test.json")

	s := &Store{entries: make(map[string]*Entry), path: path}
	s.Put("Bash", "persisted content", 17)
	s.IncrementTurn()

	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file not created: %v", err)
	}

	s2 := &Store{entries: make(map[string]*Entry), path: path}
	if err := s2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if s2.CurrentTurn() != 1 {
		t.Fatalf("want turn 1 after load, got %d", s2.CurrentTurn())
	}
	if e := s2.Get("persisted content"); e == nil {
		t.Fatal("entry missing after reload")
	}
}

func TestStore_flushAndLoadToolCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-tool-cache.json")

	s := &Store{entries: make(map[string]*Entry), toolCache: make(map[string]*Entry), path: path}
	cacheKey := `{"tool":"Bash","input":{"command":"git status"}}`
	cachedContent := "cached git status"
	h := s.PutToolCache(cacheKey, "Bash", cachedContent, len(cachedContent))

	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	s2 := &Store{entries: make(map[string]*Entry), path: path}
	if err := s2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	e := s2.GetToolCache(h)
	if e == nil {
		t.Fatal("tool cache entry missing after reload")
	}
	if e.Content != cachedContent {
		t.Fatalf("want cached content %q, got %q", cachedContent, e.Content)
	}
}

func TestStore_flushAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-atomic.json")

	s := &Store{entries: make(map[string]*Entry), path: path}
	s.Put("Bash", "data", 4)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	// tmp file must be cleaned up
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file should not remain after flush")
	}
}

func TestStore_contextUsedPct(t *testing.T) {
	s := NewEphemeral()
	if s.ContextUsedPct() != 0 {
		t.Fatal("initial context used pct must be 0")
	}
	s.SetContextUsedPct(82.5)
	if s.ContextUsedPct() != 82.5 {
		t.Fatalf("want 82.5, got %f", s.ContextUsedPct())
	}
}

func TestStore_compactRequested(t *testing.T) {
	s := NewEphemeral()
	if s.CompactRequested() {
		t.Fatal("initial compact requested must be false")
	}
	s.SetCompactRequested(true)
	if !s.CompactRequested() {
		t.Fatal("want true after SetCompactRequested(true)")
	}
	s.SetCompactRequested(false)
	if s.CompactRequested() {
		t.Fatal("want false after SetCompactRequested(false)")
	}
}

func TestStore_flushAndLoadContextUsedPct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-ctx.json")

	s := &Store{entries: make(map[string]*Entry), toolCache: make(map[string]*Entry), path: path}
	s.SetContextUsedPct(77.3)
	s.SetCompactRequested(true)
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	s2 := &Store{entries: make(map[string]*Entry), path: path}
	if err := s2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if s2.ContextUsedPct() != 77.3 {
		t.Fatalf("want 77.3, got %f", s2.ContextUsedPct())
	}
	if !s2.CompactRequested() {
		t.Fatal("compact_requested must survive flush/load")
	}
}
