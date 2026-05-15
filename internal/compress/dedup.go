package compress

import (
	"fmt"

	"github.com/mmalta/token-crunch/internal/session"
)

// Dedup applies differential deduplication against the session store.
// Returns (compressedOutput, wasCompressed).
func Dedup(store *session.Store, toolName, content string) (string, bool) {
	return DedupWithThreshold(store, toolName, content, session.NearMatchThreshold)
}

func DedupWithThreshold(store *session.Store, toolName, content string, threshold float64) (string, bool) {
	if len(content) == 0 {
		return content, false
	}

	// Exact match
	if e := store.Get(content); e != nil {
		turnAgo := store.CurrentTurn() - e.Turn
		msg := fmt.Sprintf("[unchanged — seen %d turn(s) ago by %s]", turnAgo, e.ToolName)
		if len(msg) >= len(content) {
			return content, false // replacement longer than original — not worth it
		}
		return msg, true
	}

	// Near-match: scan entries for high similarity
	if best := findNearMatch(store, content, threshold); best != nil {
		turnAgo := store.CurrentTurn() - best.Turn
		diff := session.UnifiedDiff(best.Content, content)
		header := fmt.Sprintf("[near-match from %d turn(s) ago by %s — diff only]\n", turnAgo, best.ToolName)
		out := header + diff
		if len(out) >= len(content) {
			return content, false
		}
		return out, true
	}

	return content, false
}

func findNearMatch(store *session.Store, content string, threshold float64) *session.Entry {
	var best *session.Entry
	var bestScore float64
	for _, e := range store.AllEntries() {
		if len(e.Content) == 0 {
			continue
		}
		score := session.DiffRatio(e.Content, content)
		if score >= threshold && score > bestScore {
			bestScore = score
			best = e
		}
	}
	return best
}
