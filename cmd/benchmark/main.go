// benchmark measures token-crunch savings per strategy on realistic fixture inputs.
// Run: go run ./cmd/benchmark
package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/micaelmalta/token-crunch/internal/compress"
	"github.com/micaelmalta/token-crunch/internal/session"
)

// case_ is a single benchmark scenario.
type case_ struct {
	name      string
	toolName  string
	toolInput map[string]any
	// turns is the sequence of outputs the model sees, simulating a real session.
	// Each entry is fed through the pipeline in order (earlier ones land in the store).
	turns []string
}

func main() {
	cases := []case_{
		repeatedFileRead(),
		repeatedGitStatus(),
		jsonArrayOutput(),
		stackTraceOutput(),
		fileTreeOutput(),
		testOutputPassFail(),
		logStreamDedup(),
		relevanceTrimming(),
		combinedSession(),
	}

	fmt.Printf("%-28s  %8s  %8s  %8s  %6s  %s\n",
		"Scenario", "Orig", "Final", "Saved", "Ratio", "Strategy")
	fmt.Println(strings.Repeat("─", 80))

	var sumOrig, sumFinal int
	for _, c := range cases {
		orig, final, strat := runCase(c)
		saved := orig - final
		ratio := 0.0
		if orig > 0 {
			ratio = float64(saved) / float64(orig) * 100
		}
		fmt.Printf("%-28s  %8s  %8s  %8s  %5.1f%%  %s\n",
			c.name,
			humanTokens(orig), humanTokens(final), humanTokens(saved),
			ratio, strat)
		sumOrig += orig
		sumFinal += final
	}

	fmt.Println(strings.Repeat("─", 80))
	totalSaved := sumOrig - sumFinal
	totalRatio := 0.0
	if sumOrig > 0 {
		totalRatio = float64(totalSaved) / float64(sumOrig) * 100
	}
	fmt.Printf("%-28s  %8s  %8s  %8s  %5.1f%%\n",
		"TOTAL",
		humanTokens(sumOrig), humanTokens(sumFinal), humanTokens(totalSaved),
		totalRatio)
}

// runCase runs all turns through a fresh ephemeral store and returns
// (total original tokens, total final tokens, strategy labels from last turn).
func runCase(c case_) (origTokens, finalTokens int, strategy string) {
	store := session.NewEphemeral()
	var lastStrat string
	for _, content := range c.turns {
		result := compress.Run(store, c.toolName, c.toolInput, content, compress.DefaultTokenBudget)
		origTokens += estimateTokens(content)
		finalTokens += estimateTokens(result.Output)
		if result.WasModified {
			lastStrat = strings.Join(result.Strategies, "+")
		}
	}
	if lastStrat == "" {
		lastStrat = "none"
	}
	return origTokens, finalTokens, lastStrat
}

// ── Fixtures ─────────────────────────────────────────────────────────────────

func repeatedFileRead() case_ {
	content := strings.Repeat("import React from 'react';\n", 3) +
		"export default function App() {\n" +
		strings.Repeat("  const x = useEffect(() => {}, []);\n", 20) +
		"  return <div>hello</div>;\n}\n"
	return case_{
		name:      "repeated file read (×5)",
		toolName:  "Read",
		toolInput: map[string]any{"file_path": "src/App.tsx"},
		turns:     repeat(content, 5),
	}
}

func repeatedGitStatus() case_ {
	content := `On branch main
Your branch is up to date with 'origin/main'.

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)

	modified:   internal/compress/pipeline.go
	modified:   internal/compress/structure.go
	modified:   internal/session/store.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)

	cmd/benchmark/
	scripts/replay.log

no changes added to commit (use "git add" and/or "git commit -a")`
	return case_{
		name:      "repeated git status (×4)",
		toolName:  "Bash",
		toolInput: map[string]any{"command": "git status"},
		turns:     repeat(content, 4),
	}
}

func jsonArrayOutput() case_ {
	var items []string
	for i := 1; i <= 50; i++ {
		items = append(items, fmt.Sprintf(`{"id":%d,"name":"user-%d","email":"user%d@example.com","role":"member","created_at":"2024-01-%02d"}`, i, i, i, (i%28)+1))
	}
	content := "[\n  " + strings.Join(items, ",\n  ") + "\n]"
	return case_{
		name:      "JSON array (50 items)",
		toolName:  "Bash",
		toolInput: map[string]any{"command": "curl https://api.example.com/users"},
		turns:     []string{content},
	}
}

func stackTraceOutput() case_ {
	content := `goroutine 1 [running]:
runtime/debug.Stack()
	/usr/local/go/src/runtime/debug/stack.go:24 +0x5b
github.com/micaelmalta/token-crunch/internal/compress.Run(...)
	/home/user/token-crunch/internal/compress/pipeline.go:42 +0x1a3
github.com/micaelmalta/token-crunch/internal/compress.Dedup(...)
	/home/user/token-crunch/internal/compress/dedup.go:15 +0x88
github.com/micaelmalta/token-crunch/internal/compress.findNearMatch(...)
	/home/user/token-crunch/internal/compress/dedup.go:34 +0x62
github.com/micaelmalta/token-crunch/internal/session.(*Store).AllEntries(...)
	/home/user/token-crunch/internal/session/store.go:124 +0x77
github.com/micaelmalta/token-crunch/internal/session.(*Store).Put(...)
	/home/user/token-crunch/internal/session/store.go:93 +0x4f
main.main()
	/home/user/token-crunch/cmd/token-crunch/main.go:18 +0x2c`
	return case_{
		name:      "stack trace collapse",
		toolName:  "Bash",
		toolInput: map[string]any{"command": "go run . post"},
		turns:     []string{content},
	}
}

func fileTreeOutput() case_ {
	var lines []string
	lines = append(lines, ".")
	dirs := []string{"cmd", "internal", "scripts"}
	for _, d := range dirs {
		lines = append(lines, "├── "+d+"/")
		for i := 0; i < 12; i++ {
			lines = append(lines, "│   ├── file_"+fmt.Sprintf("%02d", i)+".go")
		}
	}
	content := strings.Join(lines, "\n")
	return case_{
		name:      "file tree (>10 children)",
		toolName:  "Bash",
		toolInput: map[string]any{"command": "find . -type f"},
		turns:     []string{content},
	}
}

func testOutputPassFail() case_ {
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf("--- PASS: TestFeature%02d (0.00s)", i))
	}
	lines = append(lines, "--- FAIL: TestAuth (0.12s)")
	lines = append(lines, "    auth_test.go:88: expected 200, got 401")
	lines = append(lines, "--- FAIL: TestRateLimit (0.03s)")
	lines = append(lines, "    rate_test.go:44: expected header X-RateLimit-Remaining")
	lines = append(lines, "FAIL\tgithub.com/micaelmalta/token-crunch\t0.45s")
	content := strings.Join(lines, "\n")
	return case_{
		name:      "test output (40 pass, 2 fail)",
		toolName:  "Bash",
		toolInput: map[string]any{"command": "go test ./..."},
		turns:     []string{content},
	}
}

func logStreamDedup() case_ {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("2024-01-15 10:%02d:00 INFO  health check ok", i%60))
	}
	for i := 0; i < 5; i++ {
		lines = append(lines, fmt.Sprintf("2024-01-15 10:%02d:01 INFO  request POST /api/deploy", i))
	}
	lines = append(lines, "2024-01-15 10:05:02 INFO  deploy complete build=1234")
	content := strings.Join(lines, "\n")
	return case_{
		name:      "log stream (repeated lines)",
		toolName:  "Bash",
		toolInput: map[string]any{"command": "tail -n 50 server.log"},
		turns:     []string{content},
	}
}

func relevanceTrimming() case_ {
	// Simulates: cat of a large config file when the user asked about "database timeout".
	// Most lines are unrelated config keys; a handful are the relevant database block.
	var lines []string
	// 3000 unrelated config lines — well above the 2000-token budget
	sections := []string{"cache", "queue", "smtp", "storage", "logging", "metrics", "tracing", "auth", "session", "feature"}
	for i := range 3000 {
		sec := sections[i%len(sections)]
		lines = append(lines, fmt.Sprintf("%s.key_%03d = value_%03d", sec, i, i))
	}
	// 15 lines directly relevant to the query
	for i := range 15 {
		lines = append(lines, fmt.Sprintf("database.timeout_%d = %dms", i, 100+i*50))
	}
	content := strings.Join(lines, "\n")
	return case_{
		name:      "relevance trim (cat config)",
		toolName:  "Bash",
		toolInput: map[string]any{"command": "cat config/app.conf", "query": "database timeout"},
		turns:     []string{content},
	}
}

func combinedSession() case_ {
	// Simulates a realistic session: read a file, run tests, read the same file again,
	// check git status twice, fetch a JSON endpoint.
	fileContent := "package main\n\n" +
		strings.Repeat("// comment line padding\nfunc helper() {}\n", 30)

	gitStatus := `On branch feat/compression
Changes not staged for commit:
	modified:   internal/compress/pipeline.go
no changes added to commit`

	jsonResp := func() string {
		var items []string
		for i := 1; i <= 20; i++ {
			items = append(items, fmt.Sprintf(`{"id":%d,"status":"ok","ts":"2024-01-15T10:00:0%dZ"}`, i, i%10))
		}
		return "[" + strings.Join(items, ",") + "]"
	}()

	return case_{
		name:     "combined session (6 turns)",
		toolName: "Bash",
		toolInput: map[string]any{
			"command": "cat main.go && go test ./... && git status && curl /api/jobs",
		},
		turns: []string{
			fileContent,  // turn 1: read file
			gitStatus,    // turn 2: git status
			fileContent,  // turn 3: re-read same file  → dedup
			gitStatus,    // turn 4: git status again   → dedup
			jsonResp,     // turn 5: JSON array          → structure
			fileContent,  // turn 6: re-read again       → dedup
		},
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func estimateTokens(s string) int {
	words := strings.Fields(s)
	if len(words) == 0 {
		return 0
	}
	ratio := technicalRatio(words)
	return int(math.Ceil(float64(len(words)) / ratio))
}

func technicalRatio(words []string) float64 {
	const (
		proseRatio     = 0.75
		technicalRatio = 0.60
		sampleMax      = 40
	)
	sample := words
	if len(sample) > sampleMax {
		sample = sample[:sampleMax]
	}
	technical := 0
	for _, w := range sample {
		if isTechnicalWord(w) {
			technical++
		}
	}
	frac := float64(technical) / float64(len(sample))
	return proseRatio - frac*(proseRatio-technicalRatio)
}

func isTechnicalWord(w string) bool {
	for _, c := range w {
		switch {
		case c == '/' || c == '\\' || c == '.':
			return true
		case c == ':' || c == '(' || c == ')':
			return true
		case 'A' <= c && c <= 'Z':
			return true
		case c == '_' && len(w) > 4:
			return true
		}
	}
	return false
}

func humanTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}


