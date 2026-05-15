package compress

import (
	"fmt"
	"strings"

	"github.com/mmalta/token-crunch/internal/config"
	"github.com/mmalta/token-crunch/internal/session"
)

// Result holds the output of the compression pipeline.
type Result struct {
	Output      string
	OrigSize    int
	FinalSize   int
	Strategies  []string
	WasModified bool
	Trace       []string
}

// Run applies dedup → structural collapse → relevance trimming in order.
// Errors always pass through unmodified (lossless for failures).
func Run(store *session.Store, toolName string, toolInput map[string]any, content string, tokenBudget int) Result {
	cfg := config.Load()
	if tokenBudget > 0 {
		cfg.TokenBudget = tokenBudget
	}
	return RunWithConfig(store, toolName, toolInput, content, cfg)
}

func RunWithConfig(store *session.Store, toolName string, toolInput map[string]any, content string, cfg config.Config) Result {
	origSize := len(content)
	var trace []string

	// Lossless passthrough for error signals
	if looksLikeErrorWithKeywords(content, cfg.ErrorKeywords) {
		return Result{Output: content, OrigSize: origSize, FinalSize: origSize, Trace: []string{"error passthrough"}}
	}

	current := content
	var strategies []string

	// 1. Differential dedup
	if cfg.StrategyEnabled("dedup") {
		trace = append(trace, "dedup: enabled")
	} else {
		trace = append(trace, "dedup: disabled")
	}
	if cfg.StrategyEnabled("dedup") {
		if dedupOut, ok := DedupWithThreshold(store, toolName, current, cfg.NearMatch); ok {
			current = dedupOut
			strategies = append(strategies, "dedup")
			trace = append(trace, "dedup: applied")
		} else {
			trace = append(trace, "dedup: no match")
		}
	}

	// 2. Structural collapse (only on non-dedup'd content to avoid double-processing)
	if len(strategies) == 0 && cfg.StrategyEnabled("structure") {
		if structOut, ok := CollapseStructure(current, toolInput); ok && len(structOut) < len(current) {
			current = structOut
			strategies = append(strategies, "structure")
			trace = append(trace, "structure: applied")
		} else {
			trace = append(trace, "structure: no useful collapse")
		}
	} else if len(strategies) == 0 {
		trace = append(trace, "structure: disabled")
	}

	// 3. Relevance trimming
	queryTerms := ExtractQueryTerms(toolInput)
	if len(queryTerms) > 0 && cfg.StrategyEnabled("relevance") {
		if trimOut, ok := trimByRelevance(current, queryTerms, cfg.TokenBudget, cfg.MinRelevanceLines); ok && len(trimOut) < len(current) {
			current = trimOut
			strategies = append(strategies, "relevance")
			trace = append(trace, "relevance: applied")
		} else {
			trace = append(trace, "relevance: no useful trim")
		}
	} else if len(queryTerms) == 0 {
		trace = append(trace, "relevance: no query terms")
	} else {
		trace = append(trace, "relevance: disabled")
	}

	if len(strategies) == 0 || len(current) >= len(content) {
		store.PutWithStrategies(toolName, content, origSize, nil, cfg.StoreRaw)
		return Result{Output: content, OrigSize: origSize, FinalSize: origSize, Trace: trace}
	}

	// Store original content; compressed size is post-pipeline length (pre-header)
	store.PutWithStrategies(toolName, content, len(current), strategies, cfg.StoreRaw)

	// Prepend compression header
	stratStr := strings.Join(strategies, "+")
	origTokens := estimateTokens(content)
	finalTokens := estimateTokens(current)
	header := fmt.Sprintf("[token-crunch: %s → %s tokens | %s]\n",
		formatNum(origTokens), formatNum(finalTokens), stratStr)
	output := header + current

	return Result{
		Output:      output,
		OrigSize:    origSize,
		FinalSize:   len(output),
		Strategies:  strategies,
		WasModified: true,
		Trace:       trace,
	}
}

func looksLikeErrorWithKeywords(content string, keywords []string) bool {
	lower := strings.ToLower(content)
	if len(keywords) == 0 {
		keywords = []string{"error:", "exception:", "fatal:", "panic:", "traceback"}
	}
	for _, kw := range keywords {
		if strings.Contains(lower[:min(len(lower), 200)], kw) {
			return true
		}
	}
	return false
}

func formatNum(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
}
