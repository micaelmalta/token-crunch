package hook

import (
	"fmt"

	"github.com/micaelmalta/token-crunch/internal/config"
	"github.com/micaelmalta/token-crunch/internal/session"
)

const defaultCompactMessage = "You have written a partial transcript for the initial task above. Please write a summary of the transcript. The purpose of this summary is to provide continuity so you can continue to make progress towards solving the task in a future context, where the raw history above may not be accessible and will be replaced with this summary. Write down anything that would be helpful, including the state, next steps, learnings etc. You must wrap your summary in a <summary></summary> block."

// compactContext returns the additionalContext string to inject when a
// compaction nudge is warranted, or "" if no nudge is needed.
func compactContext(store *session.Store, cfg config.Config) string {
	if cfg.CompactThreshold <= 0 {
		return ""
	}
	pct := store.ContextUsedPct()
	if pct < cfg.CompactThreshold {
		return ""
	}
	if store.CompactRequested() {
		return ""
	}
	msg := cfg.CompactMessage
	if msg == "" {
		msg = defaultCompactMessage
	}
	store.SetCompactRequested(true)
	debugf("auto-compact triggered: context_used=%.1f%% threshold=%.1f%%", pct, cfg.CompactThreshold)
	return fmt.Sprintf("[token-crunch: auto-compact at %.0f%% context usage]\n%s", pct, msg)
}
