package hook

import (
	"encoding/json"
	"io"
	"os"

	"github.com/micaelmalta/token-crunch/internal/session"
)

type flushInput struct {
	SessionID     string         `json:"session_id"`
	ContextWindow *contextWindow `json:"context_window"`
}

type contextWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
}

// Flush handles the Stop hook — records context window usage, persists the session store to disk.
func Flush() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	capturePayload("stop", raw)

	var inp flushInput
	_ = json.Unmarshal(raw, &inp)

	session.Init(inp.SessionID)
	store := session.Global()
	store.IncrementTurn()
	if inp.ContextWindow != nil {
		store.SetContextUsedPct(inp.ContextWindow.UsedPercentage)
		debugf("flush context_window_used=%.1f%%", inp.ContextWindow.UsedPercentage)
	}
	// reset compaction flag each turn so it can re-trigger if usage stays high
	store.SetCompactRequested(false)
	return store.Flush()
}
