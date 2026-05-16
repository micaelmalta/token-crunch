package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/micaelmalta/token-crunch/internal/compress"
	"github.com/micaelmalta/token-crunch/internal/config"
	"github.com/micaelmalta/token-crunch/internal/session"
)

type postInput struct {
	SessionID string         `json:"session_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	// tool_response shape varies by tool — capture as raw JSON and extract text ourselves
	ToolResponse json.RawMessage `json:"tool_response"`
}

// toolText extracts the human-readable text content from a tool_response regardless of shape:
//   - Bash:  {"stdout": "...", "stderr": "..."}
//   - Read/Write/Edit: {"content": "..."}  or  {"text": "..."}
//   - MCP tools: {"content": [{"type":"text","text":"..."}], ...}
//   - Generic tools: {"output": "..."} (extracted but not shape-preserved)
func toolText(raw json.RawMessage) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}

	// Bash
	if stdout, ok := m["stdout"]; ok {
		var s string
		if err := json.Unmarshal(stdout, &s); err == nil {
			if isProbablyBinaryOrHugeLine(s) {
				return ""
			}
			return s
		}
		return ""
	}

	// Read / generic text
	for _, key := range []string{"content", "text"} {
		if v, ok := m[key]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				if isProbablyBinaryOrHugeLine(s) {
					return ""
				}
				return s
			}
		}
	}

	if v, ok := m["output"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			if isProbablyBinaryOrHugeLine(s) {
				return ""
			}
			return s
		}
	}

	// MCP: content array [{type, text}]
	if arr, ok := m["content"]; ok {
		var v any
		if err := json.Unmarshal(arr, &v); err == nil {
			text := strings.Join(collectText(v), "\n\n")
			if isProbablyBinaryOrHugeLine(text) {
				return ""
			}
			return text
		}
	}

	var v any
	if err := json.Unmarshal(raw, &v); err == nil {
		text := strings.Join(collectText(v), "\n\n")
		if isProbablyBinaryOrHugeLine(text) {
			return ""
		}
		return text
	}

	return ""
}

func collectText(v any) []string {
	var out []string
	switch x := v.(type) {
	case string:
		if x != "" {
			out = append(out, x)
		}
	case []any:
		for _, item := range x {
			out = append(out, collectText(item)...)
		}
	case map[string]any:
		if typ, _ := x["type"].(string); typ != "" && typ != "text" {
			return out
		}
		if typ, _ := x["type"].(string); typ == "text" {
			if text, _ := x["text"].(string); text != "" {
				out = append(out, text)
			}
			return out
		}
		seen := map[string]bool{}
		for _, key := range []string{"text", "content", "output", "stdout"} {
			if val, ok := x[key]; ok {
				out = append(out, collectText(val)...)
				seen[key] = true
			}
		}
		for key, val := range x {
			if !seen[key] {
				out = append(out, collectText(val)...)
			}
		}
	}
	return out
}

func isProbablyBinaryOrHugeLine(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsRune(s, '\x00') {
		return true
	}
	for _, line := range strings.Split(s, "\n") {
		if len(line) > 20000 && len(strings.Fields(line)) < 20 {
			return true
		}
	}
	return false
}

func updatedToolOutput(raw json.RawMessage, replacement string) (any, bool) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}

	if _, ok := m["stdout"]; ok {
		m["stdout"] = replacement
		return m, true
	}

	if v, ok := m["content"]; ok {
		switch content := v.(type) {
		case string:
			m["content"] = replacement
			return m, true
		case []any:
			replaced := false
			var next []any
			for _, item := range content {
				obj, ok := item.(map[string]any)
				if !ok {
					next = append(next, item)
					continue
				}
				if typ, _ := obj["type"].(string); typ == "text" {
					if !replaced {
						obj["text"] = replacement
						next = append(next, obj)
						replaced = true
					}
					continue
				}
				next = append(next, obj)
			}
			if !replaced {
				next = append(next, map[string]any{"type": "text", "text": replacement})
			}
			m["content"] = next
			return m, true
		}
	}

	if _, ok := m["text"]; ok {
		m["text"] = replacement
		return m, true
	}

	return nil, false
}

// isError returns true when the tool response signals failure.
func isError(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	if v, ok := m["isError"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			return b
		}
	}
	// Bash: non-empty stderr without stdout is often an error
	if stderr, ok := m["stderr"]; ok {
		if stdout, ok2 := m["stdout"]; ok2 {
			var out, err string
			_ = json.Unmarshal(stdout, &out)
			_ = json.Unmarshal(stderr, &err)
			if err != "" && out == "" {
				return true
			}
		}
	}
	return false
}

// Post handles the PostToolUse hook.
func Post() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	capturePayload("posttooluse", raw)

	var inp postInput
	if err := json.Unmarshal(raw, &inp); err != nil {
		return nil
	}

	session.Init(inp.SessionID)
	store := session.Global()
	return postWithStore(store, inp, os.Stdout, true)
}

func postWithStore(store *session.Store, inp postInput, w io.Writer, persist bool) error {
	if isError(inp.ToolResponse) {
		return nil
	}

	content := toolText(inp.ToolResponse)
	if content == "" {
		return nil
	}

	cfg := config.Load()
	if cfg.Denies(inp.ToolName) || cfg.Denies(fmt.Sprintf("%v", inp.ToolInput)) {
		cfg.StoreRaw = false
	}
	result := compress.RunWithConfig(store, inp.ToolName, inp.ToolInput, content, cfg)
	debugf("post tool=%s modified=%t strategies=%s orig=%d final=%d", inp.ToolName, result.WasModified, strings.Join(result.Strategies, "+"), result.OrigSize, result.FinalSize)
	cacheKey := buildCacheKey(inp.ToolName, inp.ToolInput)
	store.PutToolCacheWithRaw(cacheKey, inp.ToolName, result.Output, result.OrigSize, cfg.StoreRaw)

	// Persist store after every post call so the next invocation can dedup against it.
	// (Each hook call is a fresh process — state only survives via disk.)
	if persist {
		_ = store.Flush()
	}

	if !result.WasModified {
		return nil
	}

	type hookOut struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext,omitempty"`
			UpdatedToolOutput any    `json:"updatedToolOutput,omitempty"`
		} `json:"hookSpecificOutput"`
	}
	var out hookOut
	out.HookSpecificOutput.HookEventName = "PostToolUse"
	if updated, ok := updatedToolOutput(inp.ToolResponse, result.Output); ok {
		out.HookSpecificOutput.UpdatedToolOutput = updated
	} else {
		out.HookSpecificOutput.AdditionalContext = result.Output
	}
	return json.NewEncoder(w).Encode(out)
}
