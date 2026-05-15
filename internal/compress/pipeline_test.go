package compress

import (
	"strings"
	"testing"

	"github.com/mmalta/token-crunch/internal/config"
	"github.com/mmalta/token-crunch/internal/session"
)

func TestPipeline_errorPassthrough(t *testing.T) {
	s := session.NewEphemeral()
	content := "error: connection refused\nsome detail"
	r := Run(s, "Bash", nil, content, 2000)
	if r.WasModified {
		t.Fatal("error output must pass through unmodified")
	}
	if r.Output != content {
		t.Fatal("error output must be identical to input")
	}
}

func TestPipeline_emptyContent(t *testing.T) {
	s := session.NewEphemeral()
	r := Run(s, "Bash", nil, "", 2000)
	if r.WasModified || r.Output != "" {
		t.Fatal("empty content must pass through unchanged")
	}
}

func TestPipeline_dedupOnSecondCall(t *testing.T) {
	s := session.NewEphemeral()
	content := strings.Repeat("git status line\n", 30)

	r1 := Run(s, "Bash", map[string]any{"command": "git status"}, content, 2000)
	if r1.WasModified {
		t.Fatal("first call must not be modified")
	}

	r2 := Run(s, "Bash", map[string]any{"command": "git status"}, content, 2000)
	if !r2.WasModified {
		t.Fatal("second call with same content must be deduped")
	}
	if !strings.Contains(strings.Join(r2.Strategies, ","), "dedup") {
		t.Fatalf("expected dedup strategy, got: %v", r2.Strategies)
	}
}

func TestPipeline_structureOnFirstCall(t *testing.T) {
	s := session.NewEphemeral()
	items := make([]string, 10)
	for i := range items {
		items[i] = `{"id":` + itoa(i) + `,"name":"user"}`
	}
	content := "[" + strings.Join(items, ",") + "]"

	r := Run(s, "Bash", nil, content, 2000)
	if !r.WasModified {
		t.Fatal("JSON array must be structurally collapsed")
	}
	if !strings.Contains(strings.Join(r.Strategies, ","), "structure") {
		t.Fatalf("expected structure strategy, got: %v", r.Strategies)
	}
}

func TestPipeline_headerFormat(t *testing.T) {
	s := session.NewEphemeral()
	items := make([]string, 8)
	for i := range items {
		items[i] = `{"id":` + itoa(i) + `}`
	}
	content := "[" + strings.Join(items, ",") + "]"

	r := Run(s, "Bash", nil, content, 2000)
	if !strings.HasPrefix(r.Output, "[token-crunch:") {
		t.Fatalf("output must start with header, got: %s", r.Output[:min(50, len(r.Output))])
	}
	if !strings.Contains(r.Output, "tokens") {
		t.Fatal("header must contain token counts")
	}
	if !strings.Contains(r.Output, "|") {
		t.Fatal("header must contain strategy separator")
	}
}

func TestPipeline_storesOnFirstCall(t *testing.T) {
	s := session.NewEphemeral()
	content := strings.Repeat("unique output line\n", 5)
	Run(s, "Read", nil, content, 2000)

	if e := s.Get(content); e == nil {
		t.Fatal("pipeline must store content in session after first call")
	}
}

func TestPipeline_relevanceTrims(t *testing.T) {
	s := session.NewEphemeral()
	var lines []string
	for range 500 {
		lines = append(lines, "unrelated.setting = value")
	}
	for range 5 {
		lines = append(lines, "database.timeout = 5000ms")
	}
	content := strings.Join(lines, "\n")

	r := Run(s, "Bash", map[string]any{"command": "database timeout"}, content, 100)
	if !r.WasModified {
		t.Fatal("large over-budget content must be trimmed")
	}
	if !strings.Contains(strings.Join(r.Strategies, ","), "relevance") {
		t.Fatalf("expected relevance strategy, got: %v", r.Strategies)
	}
}

func TestPipeline_dedupSkipsStructure(t *testing.T) {
	// Once dedup fires, structure must not also fire on the same content
	s := session.NewEphemeral()
	items := make([]string, 10)
	for i := range items {
		items[i] = `{"id":` + itoa(i) + `}`
	}
	content := "[" + strings.Join(items, ",") + "]"

	Run(s, "Bash", nil, content, 2000) // prime the store

	r := Run(s, "Bash", nil, content, 2000)
	for _, strat := range r.Strategies {
		if strat == "structure" {
			t.Fatal("structure must not fire when dedup already matched")
		}
	}
}

func TestPipeline_configDisablesStructure(t *testing.T) {
	s := session.NewEphemeral()
	items := make([]string, 10)
	for i := range items {
		items[i] = `{"id":` + itoa(i) + `}`
	}
	content := "[" + strings.Join(items, ",") + "]"

	r := RunWithConfig(s, "Bash", nil, content, config.Config{
		TokenBudget:       2000,
		NearMatch:         0.85,
		MinRelevanceLines: 10,
		ErrorKeywords:     []string{"error:"},
		Strategies:        map[string]bool{"dedup": true, "structure": false, "relevance": true},
		StoreRaw:          true,
	})
	if r.WasModified {
		t.Fatalf("structure-disabled JSON should pass through unchanged, got strategies %v", r.Strategies)
	}
}
