package hook

import (
	"encoding/json"
	"io"
	"os"

	"github.com/mmalta/token-crunch/internal/session"
)

type flushInput struct {
	SessionID string `json:"session_id"`
}

// Flush handles the Stop hook — persists the session store to disk.
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
	return store.Flush()
}
