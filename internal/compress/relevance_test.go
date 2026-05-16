package compress

import (
	"strings"
	"testing"
)

func TestTrimByRelevance_belowBudget(t *testing.T) {
	content := strings.Repeat("some line of text\n", 5)
	out, ok := TrimByRelevance(content, []string{"text"}, 2000)
	if ok || out != content {
		t.Fatal("content under budget must pass through unchanged")
	}
}

func TestTrimByRelevance_tooFewLines(t *testing.T) {
	// 9 lines — below the 10-line minimum
	content := strings.Repeat("line\n", 9)
	out, ok := TrimByRelevance(content, []string{"line"}, 1)
	if ok || out != content {
		t.Fatal("fewer than 10 lines must not be trimmed")
	}
}

func TestTrimByRelevance_dropsIrrelevant(t *testing.T) {
	var lines []string
	for range 400 {
		lines = append(lines, "cache.key_001 = value_001")
	}
	for range 10 {
		lines = append(lines, "database.timeout = 5000ms")
	}
	content := strings.Join(lines, "\n")

	out, ok := TrimByRelevance(content, []string{"database", "timeout"}, 200)
	if !ok {
		t.Fatal("large content over budget must be trimmed")
	}
	if len(out) >= len(content) {
		t.Fatal("trimmed output must be shorter")
	}
	if !strings.Contains(out, "database.timeout") {
		t.Fatal("relevant lines must be preserved")
	}
	if !strings.Contains(out, "omitted") {
		t.Fatal("omission marker must appear")
	}
}

func TestTrimByRelevance_preservesFirstAndLast(t *testing.T) {
	var lines []string
	lines = append(lines, "FIRST LINE")
	for range 200 {
		lines = append(lines, "irrelevant filler content padding words here")
	}
	lines = append(lines, "LAST LINE")
	content := strings.Join(lines, "\n")

	out, ok := TrimByRelevance(content, []string{"needle"}, 50)
	if !ok {
		t.Fatal("must trim over-budget content")
	}
	if !strings.HasPrefix(out, "FIRST LINE") {
		t.Fatal("first line must always be preserved")
	}
	if !strings.Contains(out, "LAST LINE") {
		t.Fatal("last line must always be preserved")
	}
}

func TestTrimByRelevance_preservesExactQueryMatch(t *testing.T) {
	var lines []string
	for range 200 {
		lines = append(lines, "unrelated content abc def ghi")
	}
	lines = append(lines, "authMiddleware wired at /api/v2")
	lines = append(lines, "more unrelated content xyz")
	content := strings.Join(lines, "\n")

	out, _ := TrimByRelevance(content, []string{"authMiddleware"}, 50)
	if !strings.Contains(out, "authMiddleware wired at /api/v2") {
		t.Fatal("line containing exact query term must be preserved")
	}
}

func TestExtractQueryTerms(t *testing.T) {
	input := map[string]any{
		"command": "grep -r authMiddleware .",
		"flag":    "--include=*.go",
	}
	terms := ExtractQueryTerms(input)
	joined := strings.Join(terms, " ")
	if !strings.Contains(joined, "authMiddleware") {
		t.Fatalf("expected authMiddleware in terms, got: %v", terms)
	}
	if !strings.Contains(joined, "grep") {
		t.Fatalf("expected grep in terms, got: %v", terms)
	}
}

func TestEstimateTokens_empty(t *testing.T) {
	if estimateTokens("") != 0 {
		t.Fatal("empty string must be 0 tokens")
	}
}

func TestEstimateTokens_words(t *testing.T) {
	// 3 plain words → no technical markers → ratio 0.75 → ceil(3/0.75) = 4
	n := estimateTokens("one two three")
	if n != 4 {
		t.Fatalf("want 4 tokens for 3 plain words, got %d", n)
	}
}

func TestEstimateTokens_technical(t *testing.T) {
	// Same number of words; technical markers push ratio below 0.75 → more tokens.
	plain := estimateTokens("one two three")      // 3 words, ratio 0.75 → 4 tokens
	technical := estimateTokens("runtime/debug.Stack() pipeline.go:42 internal/compress.Run(...)")
	if technical <= plain {
		t.Fatalf("technical content (%d tokens) must exceed plain prose (%d tokens) for equal word count", technical, plain)
	}
}

func TestEstimateTokens_minifiedJSON(t *testing.T) {
	// {"id":1,"name":"alpha"} is one whitespace-word but contains many punctuation tokens.
	// The punctuation bonus must push the estimate well above 1.
	n := estimateTokens(`{"id":1,"name":"alpha"}`)
	if n < 5 {
		t.Fatalf("minified JSON must count more than 1 token, got %d", n)
	}
}

func TestEstimateTokens_punctBonus_additive(t *testing.T) {
	// Without the punctuation bonus, minified JSON (1 whitespace-word) would be
	// estimated as 1-2 tokens while spaced JSON (9 words) lands at ~17 tokens —
	// an ~8-10x gap.  The punctuation bonus must shrink that gap to within 4x.
	spaced := estimateTokens(`{ "id" : 1 , "name" : "alpha" }`)
	minified := estimateTokens(`{"id":1,"name":"alpha"}`)
	if spaced > minified*4 {
		t.Fatalf("spaced (%d) vs minified (%d) gap exceeds 4x; punctuation bonus not working", spaced, minified)
	}
}
