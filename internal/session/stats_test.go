package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var baseTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// captureStdout runs f and returns everything written to os.Stdout during the call.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = orig

	var sb strings.Builder
	buf := make([]byte, 8192)
	for {
		n, _ := r.Read(buf)
		if n == 0 {
			break
		}
		sb.Write(buf[:n])
	}
	r.Close()
	return sb.String()
}

// writeSessionFile writes a minimal session JSON file to dir/token-crunch/.
func writeSessionFile(t *testing.T, dir, name string, entries map[string]*Entry) {
	t.Helper()
	storeDir := filepath.Join(dir, "token-crunch")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Assign distinct StoredAt timestamps so timeline sort is deterministic.
	i := 0
	for _, e := range entries {
		if e.StoredAt.IsZero() {
			e.StoredAt = baseTime.Add(time.Duration(i) * time.Minute)
		}
		i++
	}
	data, err := json.Marshal(diskFormat{Turn: 1, Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ── humanBytes ────────────────────────────────────────────────────────────────

func TestHumanBytes_bytes(t *testing.T) {
	if got := humanBytes(512); got != "512B" {
		t.Fatalf("want 512B, got %s", got)
	}
}

func TestHumanBytes_kilobytes(t *testing.T) {
	if got := humanBytes(2048); got != "2.0KB" {
		t.Fatalf("want 2.0KB, got %s", got)
	}
}

func TestHumanBytes_megabytes(t *testing.T) {
	if got := humanBytes(3 * 1 << 20); got != "3.0MB" {
		t.Fatalf("want 3.0MB, got %s", got)
	}
}

func TestHumanBytes_zero(t *testing.T) {
	if got := humanBytes(0); got != "0B" {
		t.Fatalf("want 0B, got %s", got)
	}
}

func TestHumanBytes_exactKB(t *testing.T) {
	if got := humanBytes(1024); got != "1.0KB" {
		t.Fatalf("want 1.0KB, got %s", got)
	}
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func TestStats_noSessionDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	// No token-crunch subdir created — should print "No sessions found." without error
	out := captureStdout(t, func() {
		if err := Stats(); err != nil {
			t.Fatalf("Stats: %v", err)
		}
	})
	if !strings.Contains(out, "No sessions found") {
		t.Fatalf("expected 'No sessions found', got: %s", out)
	}
}

func TestStats_emptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "token-crunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := Stats(); err != nil {
			t.Fatalf("Stats: %v", err)
		}
	})
	if !strings.Contains(out, "No sessions found") {
		t.Fatalf("expected 'No sessions found', got: %s", out)
	}
}

func TestStats_singleSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	writeSessionFile(t, dir, "session-abc.json", map[string]*Entry{
		"hash1": {Hash: "hash1", ToolName: "Bash", OriginalSize: 1000, CompressedSize: 200},
		"hash2": {Hash: "hash2", ToolName: "Read", OriginalSize: 500, CompressedSize: 100},
	})

	out := captureStdout(t, func() {
		if err := Stats(); err != nil {
			t.Fatalf("Stats: %v", err)
		}
	})

	if !strings.Contains(out, "session-abc.json") {
		t.Fatalf("expected session filename in output, got:\n%s", out)
	}
	if !strings.Contains(out, "TOTAL") {
		t.Fatalf("expected TOTAL row, got:\n%s", out)
	}
	// 1500 orig, 300 comp → 80% savings
	if !strings.Contains(out, "80.0%") {
		t.Fatalf("expected 80.0%% savings ratio, got:\n%s", out)
	}
}

func TestStats_reportsStrategyToolAndCacheMetrics(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	storeDir := filepath.Join(dir, "token-crunch")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(diskFormat{
		Turn: 1,
		Entries: map[string]*Entry{
			"h1": {Hash: "h1", ToolName: "Bash", OriginalSize: 1000, CompressedSize: 100, Strategies: []string{"structure"}},
			"h2": {Hash: "h2", ToolName: "Read", OriginalSize: 500, CompressedSize: 250, Strategies: []string{"dedup"}},
		},
		Metrics: Metrics{PreCacheHits: 2, PreCacheMisses: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "session-metrics.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := Stats(); err != nil {
			t.Fatalf("Stats: %v", err)
		}
	})
	for _, want := range []string{"PreToolUse cache", "structure", "dedup", "Bash", "Read"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestStatsJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	writeSessionFile(t, dir, "session-json.json", map[string]*Entry{
		"h1": {Hash: "h1", ToolName: "Bash", OriginalSize: 1000, CompressedSize: 500, Strategies: []string{"structure"}},
	})

	out := captureStdout(t, func() {
		if err := StatsJSON(); err != nil {
			t.Fatalf("StatsJSON: %v", err)
		}
	})
	var report StatsReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stats JSON must parse: %v\n%s", err, out)
	}
	if report.Total.Original != 1000 || report.Total.Compressed != 500 {
		t.Fatalf("unexpected totals: %+v", report.Total)
	}
	if len(report.TopOutputs) != 1 || len(report.Timeline) != 1 {
		t.Fatalf("expected top output and timeline entry: %+v", report)
	}
}

func TestStats_multipleSessions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	writeSessionFile(t, dir, "session-s1.json", map[string]*Entry{
		"h1": {Hash: "h1", OriginalSize: 2000, CompressedSize: 1000},
	})
	writeSessionFile(t, dir, "session-s2.json", map[string]*Entry{
		"h2": {Hash: "h2", OriginalSize: 4000, CompressedSize: 1000},
	})

	out := captureStdout(t, func() {
		if err := Stats(); err != nil {
			t.Fatalf("Stats: %v", err)
		}
	})

	if !strings.Contains(out, "2 session(s)") {
		t.Fatalf("expected '2 session(s)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "session-s1.json") || !strings.Contains(out, "session-s2.json") {
		t.Fatalf("expected both session filenames, got:\n%s", out)
	}
}

func TestStats_skipsTmpFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	storeDir := filepath.Join(dir, "token-crunch")
	os.MkdirAll(storeDir, 0o755)
	// Write a lingering .tmp file — must be ignored
	os.WriteFile(filepath.Join(storeDir, "session-x.json.tmp"), []byte(`{}`), 0o644)

	out := captureStdout(t, func() {
		if err := Stats(); err != nil {
			t.Fatalf("Stats: %v", err)
		}
	})
	if !strings.Contains(out, "No sessions found") {
		t.Fatalf("expected 'No sessions found' when only .tmp files present, got:\n%s", out)
	}
}

func TestStats_skipsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	storeDir := filepath.Join(dir, "token-crunch")
	os.MkdirAll(storeDir, 0o755)
	os.WriteFile(filepath.Join(storeDir, "session-bad.json"), []byte(`not json`), 0o644)

	// Malformed file is silently skipped
	out := captureStdout(t, func() {
		if err := Stats(); err != nil {
			t.Fatalf("Stats: %v", err)
		}
	})
	if !strings.Contains(out, "No sessions found") {
		t.Fatalf("expected 'No sessions found' when only malformed files present, got:\n%s", out)
	}
}

func TestClean_removesOldSessions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	storeDir := filepath.Join(dir, "token-crunch")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(storeDir, "session-old.json")
	newPath := filepath.Join(storeDir, "session-new.json")
	if err := os.WriteFile(oldPath, []byte(`{"entries":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(`{"entries":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := Clean(5); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("old session should be removed")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal("new session should remain")
	}
}
