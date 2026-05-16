package hook

import (
	"encoding/json"
	"io"
	"os"

	"github.com/micaelmalta/token-crunch/internal/config"
	"github.com/micaelmalta/token-crunch/internal/session"
)

type preInput struct {
	SessionID string         `json:"session_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

// Pre handles the PreToolUse hook.
// Cache hit → injects the stored result via additionalContext.
// Cache miss → writes nothing (tool runs normally).
func Pre() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	capturePayload("pretooluse", raw)

	var inp preInput
	if err := json.Unmarshal(raw, &inp); err != nil {
		return nil
	}

	session.Init(inp.SessionID)
	store := session.Global()
	return preWithStore(store, inp, os.Stdout)
}

type preHookOut struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext,omitempty"`
	} `json:"hookSpecificOutput"`
}

func preWithStore(store *session.Store, inp preInput, w io.Writer) error {
	cfg := config.Load()
	cacheKey := buildCacheKey(inp.ToolName, inp.ToolInput)

	if e := store.GetToolCache(session.Hash(cacheKey)); e != nil {
		store.RecordPreCacheHit()
		debugf("pre cache hit tool=%s", inp.ToolName)
		var out preHookOut
		out.HookSpecificOutput.HookEventName = "PreToolUse"
		out.HookSpecificOutput.AdditionalContext = e.Content
		if nudge := compactContext(store, cfg); nudge != "" {
			out.HookSpecificOutput.AdditionalContext = nudge + "\n\n" + out.HookSpecificOutput.AdditionalContext
			_ = store.Flush()
		}
		return json.NewEncoder(w).Encode(out)
	}

	store.RecordPreCacheMiss()
	debugf("pre cache miss tool=%s", inp.ToolName)

	if nudge := compactContext(store, cfg); nudge != "" {
		var out preHookOut
		out.HookSpecificOutput.HookEventName = "PreToolUse"
		out.HookSpecificOutput.AdditionalContext = nudge
		_ = store.Flush()
		return json.NewEncoder(w).Encode(out)
	}

	return nil
}

func buildCacheKey(toolName string, input map[string]any) string {
	b, _ := json.Marshal(struct {
		Tool  string         `json:"tool"`
		Input map[string]any `json:"input"`
	}{Tool: toolName, Input: input})
	return string(b)
}
