package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TokenBudget       int
	NearMatch         float64
	MinRelevanceLines int
	ErrorKeywords     []string
	Strategies        map[string]bool
	MaxSessionBytes   int64
	RetentionDays     int
	StoreRaw          bool
	Debug             bool
	Denylist          []string
	CompactThreshold  float64
	CompactMessage    string
}

func Load() Config {
	cfg := Config{
		TokenBudget:       envInt("TOKEN_CRUNCH_TOKEN_BUDGET", 2000),
		NearMatch:         envFloat("TOKEN_CRUNCH_NEAR_MATCH", 0.85),
		MinRelevanceLines: envInt("TOKEN_CRUNCH_MIN_RELEVANCE_LINES", 10),
		ErrorKeywords:     envList("TOKEN_CRUNCH_ERROR_KEYWORDS", []string{"error:", "exception:", "fatal:", "panic:", "traceback"}),
		Strategies:        envStrategies(),
		MaxSessionBytes:   int64(envInt("TOKEN_CRUNCH_MAX_SESSION_BYTES", 10<<20)),
		RetentionDays:     envInt("TOKEN_CRUNCH_RETENTION_DAYS", 30),
		StoreRaw:          envBool("TOKEN_CRUNCH_STORE_RAW", true),
		Debug:             envBool("TOKEN_CRUNCH_DEBUG", false),
		Denylist:          envList("TOKEN_CRUNCH_DENYLIST", nil),
		CompactThreshold:  envFloat("TOKEN_CRUNCH_COMPACT_THRESHOLD", 75),
		CompactMessage:    envString("TOKEN_CRUNCH_COMPACT_MESSAGE", ""),
	}
	if path := strings.TrimSpace(os.Getenv("TOKEN_CRUNCH_CONFIG")); path != "" {
		cfg = applyConfigFile(cfg, path)
		cfg = applyEnvOverrides(cfg)
	}
	return cfg
}

func (c Config) StrategyEnabled(name string) bool {
	if c.Strategies == nil {
		return true
	}
	return c.Strategies[name]
}

func (c Config) Denies(s string) bool {
	s = strings.ToLower(s)
	for _, pat := range c.Denylist {
		pat = strings.ToLower(strings.TrimSpace(pat))
		if pat != "" && strings.Contains(s, pat) {
			return true
		}
	}
	return false
}

type fileConfig struct {
	TokenBudget       *int     `json:"token_budget"`
	NearMatch         *float64 `json:"near_match"`
	MinRelevanceLines *int     `json:"min_relevance_lines"`
	ErrorKeywords     []string `json:"error_keywords"`
	Strategies        []string `json:"strategies"`
	MaxSessionBytes   *int64   `json:"max_session_bytes"`
	RetentionDays     *int     `json:"retention_days"`
	StoreRaw          *bool    `json:"store_raw"`
	Debug             *bool    `json:"debug"`
	Denylist          []string `json:"denylist"`
	CompactThreshold  *float64 `json:"compact_threshold"`
	CompactMessage    *string  `json:"compact_message"`
}

func applyConfigFile(cfg Config, path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return cfg
	}
	if fc.TokenBudget != nil {
		cfg.TokenBudget = *fc.TokenBudget
	}
	if fc.NearMatch != nil {
		cfg.NearMatch = *fc.NearMatch
	}
	if fc.MinRelevanceLines != nil {
		cfg.MinRelevanceLines = *fc.MinRelevanceLines
	}
	if len(fc.ErrorKeywords) > 0 {
		cfg.ErrorKeywords = fc.ErrorKeywords
	}
	if len(fc.Strategies) > 0 {
		cfg.Strategies = map[string]bool{"dedup": false, "structure": false, "relevance": false}
		for _, s := range fc.Strategies {
			cfg.Strategies[strings.ToLower(strings.TrimSpace(s))] = true
		}
	}
	if fc.MaxSessionBytes != nil {
		cfg.MaxSessionBytes = *fc.MaxSessionBytes
	}
	if fc.RetentionDays != nil {
		cfg.RetentionDays = *fc.RetentionDays
	}
	if fc.StoreRaw != nil {
		cfg.StoreRaw = *fc.StoreRaw
	}
	if fc.Debug != nil {
		cfg.Debug = *fc.Debug
	}
	if len(fc.Denylist) > 0 {
		cfg.Denylist = fc.Denylist
	}
	if fc.CompactThreshold != nil {
		cfg.CompactThreshold = *fc.CompactThreshold
	}
	if fc.CompactMessage != nil {
		cfg.CompactMessage = *fc.CompactMessage
	}
	return cfg
}

func applyEnvOverrides(cfg Config) Config {
	if os.Getenv("TOKEN_CRUNCH_TOKEN_BUDGET") != "" {
		cfg.TokenBudget = envInt("TOKEN_CRUNCH_TOKEN_BUDGET", cfg.TokenBudget)
	}
	if os.Getenv("TOKEN_CRUNCH_NEAR_MATCH") != "" {
		cfg.NearMatch = envFloat("TOKEN_CRUNCH_NEAR_MATCH", cfg.NearMatch)
	}
	if os.Getenv("TOKEN_CRUNCH_MIN_RELEVANCE_LINES") != "" {
		cfg.MinRelevanceLines = envInt("TOKEN_CRUNCH_MIN_RELEVANCE_LINES", cfg.MinRelevanceLines)
	}
	if os.Getenv("TOKEN_CRUNCH_ERROR_KEYWORDS") != "" {
		cfg.ErrorKeywords = envList("TOKEN_CRUNCH_ERROR_KEYWORDS", cfg.ErrorKeywords)
	}
	if os.Getenv("TOKEN_CRUNCH_STRATEGIES") != "" {
		cfg.Strategies = envStrategies()
	}
	if os.Getenv("TOKEN_CRUNCH_MAX_SESSION_BYTES") != "" {
		cfg.MaxSessionBytes = int64(envInt("TOKEN_CRUNCH_MAX_SESSION_BYTES", int(cfg.MaxSessionBytes)))
	}
	if os.Getenv("TOKEN_CRUNCH_RETENTION_DAYS") != "" {
		cfg.RetentionDays = envInt("TOKEN_CRUNCH_RETENTION_DAYS", cfg.RetentionDays)
	}
	if os.Getenv("TOKEN_CRUNCH_STORE_RAW") != "" {
		cfg.StoreRaw = envBool("TOKEN_CRUNCH_STORE_RAW", cfg.StoreRaw)
	}
	if os.Getenv("TOKEN_CRUNCH_DEBUG") != "" {
		cfg.Debug = envBool("TOKEN_CRUNCH_DEBUG", cfg.Debug)
	}
	if os.Getenv("TOKEN_CRUNCH_DENYLIST") != "" {
		cfg.Denylist = envList("TOKEN_CRUNCH_DENYLIST", cfg.Denylist)
	}
	if os.Getenv("TOKEN_CRUNCH_COMPACT_THRESHOLD") != "" {
		cfg.CompactThreshold = envFloat("TOKEN_CRUNCH_COMPACT_THRESHOLD", cfg.CompactThreshold)
	}
	if os.Getenv("TOKEN_CRUNCH_COMPACT_MESSAGE") != "" {
		cfg.CompactMessage = envString("TOKEN_CRUNCH_COMPACT_MESSAGE", cfg.CompactMessage)
	}
	return cfg
}

func envStrategies() map[string]bool {
	raw := strings.TrimSpace(os.Getenv("TOKEN_CRUNCH_STRATEGIES"))
	out := map[string]bool{"dedup": true, "structure": true, "relevance": true}
	if raw == "" {
		return out
	}
	for k := range out {
		out[k] = false
	}
	for _, p := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(p))
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func envList(name string, def []string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func envInt(name string, def int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func envFloat(name string, def float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return def
	}
	return n
}

func envString(name string, def string) string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	return raw
}

func envBool(name string, def bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch raw {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
