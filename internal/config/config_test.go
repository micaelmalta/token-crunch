package config

import "testing"
import "os"
import "path/filepath"

func TestLoad_envOverrides(t *testing.T) {
	t.Setenv("TOKEN_CRUNCH_TOKEN_BUDGET", "123")
	t.Setenv("TOKEN_CRUNCH_NEAR_MATCH", "0.7")
	t.Setenv("TOKEN_CRUNCH_MIN_RELEVANCE_LINES", "4")
	t.Setenv("TOKEN_CRUNCH_STRATEGIES", "dedup,relevance")
	t.Setenv("TOKEN_CRUNCH_ERROR_KEYWORDS", "boom:,oops")
	t.Setenv("TOKEN_CRUNCH_STORE_RAW", "false")
	t.Setenv("TOKEN_CRUNCH_DEBUG", "true")

	cfg := Load()
	if cfg.TokenBudget != 123 {
		t.Fatalf("want token budget 123, got %d", cfg.TokenBudget)
	}
	if cfg.NearMatch != 0.7 {
		t.Fatalf("want near match 0.7, got %f", cfg.NearMatch)
	}
	if cfg.MinRelevanceLines != 4 {
		t.Fatalf("want min relevance lines 4, got %d", cfg.MinRelevanceLines)
	}
	if !cfg.StrategyEnabled("dedup") || !cfg.StrategyEnabled("relevance") || cfg.StrategyEnabled("structure") {
		t.Fatalf("unexpected strategy config: %+v", cfg.Strategies)
	}
	if cfg.ErrorKeywords[0] != "boom:" || cfg.ErrorKeywords[1] != "oops" {
		t.Fatalf("unexpected error keywords: %+v", cfg.ErrorKeywords)
	}
	if cfg.StoreRaw {
		t.Fatal("store raw must be false")
	}
	if !cfg.Debug {
		t.Fatal("debug must be true")
	}
}

func TestLoad_configFileAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token-crunch.json")
	if err := os.WriteFile(path, []byte(`{
		"token_budget": 321,
		"near_match": 0.66,
		"strategies": ["structure"],
		"store_raw": false,
		"denylist": ["secret"]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOKEN_CRUNCH_CONFIG", path)
	t.Setenv("TOKEN_CRUNCH_TOKEN_BUDGET", "654")

	cfg := Load()
	if cfg.TokenBudget != 654 {
		t.Fatalf("env must override config file token budget, got %d", cfg.TokenBudget)
	}
	if cfg.NearMatch != 0.66 {
		t.Fatalf("want file near match 0.66, got %f", cfg.NearMatch)
	}
	if !cfg.StrategyEnabled("structure") || cfg.StrategyEnabled("dedup") {
		t.Fatalf("unexpected strategies: %+v", cfg.Strategies)
	}
	if cfg.StoreRaw {
		t.Fatal("store_raw from config file must apply")
	}
	if !cfg.Denies("/tmp/secret.txt") {
		t.Fatal("denylist pattern must match")
	}
}
