package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hookCommand and hookEntry are used only when constructing new entries to append.
// Existing entries are kept as json.RawMessage so unknown fields (e.g. "if") round-trip untouched.
type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookEntry struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

// probeCommands extracts the command strings from a raw hook-entry object without
// deserializing it fully, so unknown fields are never discarded.
type probeEntry struct {
	Hooks []struct {
		Command string `json:"command"`
	} `json:"hooks"`
}

const globalSettingsPath = ".claude/settings.json"
const localSettingsPath = ".claude/settings.local.json"

func claudeSettingsPath(local bool) string {
	if local {
		return localSettingsPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, globalSettingsPath)
}

// Install merges token-crunch hooks into ~/.claude/settings.json non-destructively.
// Pass local=true to install into ./.claude/settings.local.json instead.
func Install(local bool) error {
	path := claudeSettingsPath(local)
	raw := loadOrEmpty(path)

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		top = map[string]json.RawMessage{}
	}

	// Keep hooks as a map so unknown event types (Notification, UserPromptSubmit, etc.) round-trip untouched.
	hooks := loadHooksMap(top)

	hooks["PreToolUse"] = mergeHookRaw(hooks["PreToolUse"], hookEntry{
		Matcher: ".*",
		Hooks:   []hookCommand{{Type: "command", Command: "token-crunch pre"}},
	})
	hooks["PostToolUse"] = mergeHookRaw(hooks["PostToolUse"], hookEntry{
		Matcher: ".*",
		Hooks:   []hookCommand{{Type: "command", Command: "token-crunch post"}},
	})
	hooks["Stop"] = mergeHookRaw(hooks["Stop"], hookEntry{
		Hooks: []hookCommand{{Type: "command", Command: "token-crunch flush"}},
	})

	if err := storeHooksMap(top, hooks); err != nil {
		return err
	}
	return writeAtomic(path, top)
}

// Uninstall removes token-crunch hooks from the settings file.
// Pass local=true to target ./.claude/settings.local.json instead of the global file.
func Uninstall(local bool) error {
	path := claudeSettingsPath(local)
	raw := loadOrEmpty(path)

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil // nothing to do
	}

	hooks := loadHooksMap(top)

	hooks["PreToolUse"] = removeHookRaw(hooks["PreToolUse"], "token-crunch pre")
	hooks["PostToolUse"] = removeHookRaw(hooks["PostToolUse"], "token-crunch post")
	hooks["Stop"] = removeHookRaw(hooks["Stop"], "token-crunch flush")

	if err := storeHooksMap(top, hooks); err != nil {
		return err
	}
	return writeAtomic(path, top)
}

// loadHooksMap returns the "hooks" value as a map keyed by event type.
// Unknown event types are preserved as raw JSON so they round-trip untouched.
func loadHooksMap(top map[string]json.RawMessage) map[string]json.RawMessage {
	hooks := map[string]json.RawMessage{}
	if raw, ok := top["hooks"]; ok {
		_ = json.Unmarshal(raw, &hooks)
	}
	return hooks
}

func storeHooksMap(top map[string]json.RawMessage, hooks map[string]json.RawMessage) error {
	raw, err := marshalNoEscape(hooks)
	if err != nil {
		return err
	}
	top["hooks"] = raw
	return nil
}

func mergeHookRaw(existing json.RawMessage, entry hookEntry) json.RawMessage {
	var rawEntries []json.RawMessage
	if existing != nil {
		_ = json.Unmarshal(existing, &rawEntries)
	}
	cmd := entry.Hooks[0].Command
	for _, re := range rawEntries {
		var p probeEntry
		_ = json.Unmarshal(re, &p)
		for _, h := range p.Hooks {
			if h.Command == cmd {
				return existing // already present, return original bytes unchanged
			}
		}
	}
	newEntry, _ := marshalNoEscape(entry)
	rawEntries = append(rawEntries, newEntry)
	raw, _ := marshalNoEscape(rawEntries)
	return raw
}

func removeHookRaw(existing json.RawMessage, cmd string) json.RawMessage {
	if existing == nil {
		return nil
	}
	var rawEntries []json.RawMessage
	_ = json.Unmarshal(existing, &rawEntries)
	var out []json.RawMessage
	for _, re := range rawEntries {
		var p probeEntry
		_ = json.Unmarshal(re, &p)
		// Filter out the target command from hooks; if any remain, keep the entry.
		var keep []struct {
			Command string `json:"command"`
		}
		for _, h := range p.Hooks {
			if h.Command != cmd {
				keep = append(keep, h)
			}
		}
		if len(keep) == len(p.Hooks) {
			// Entry untouched — preserve original bytes exactly.
			out = append(out, re)
		} else if len(keep) > 0 {
			// Some hooks remain — must rewrite this entry, accepting the field loss tradeoff
			// (only our own entries ever reach this path since we only remove token-crunch commands).
			var full hookEntry
			_ = json.Unmarshal(re, &full)
			var filtered []hookCommand
			for _, h := range full.Hooks {
				if h.Command != cmd {
					filtered = append(filtered, h)
				}
			}
			full.Hooks = filtered
			rewritten, _ := marshalNoEscape(full)
			out = append(out, rewritten)
		}
		// len(keep) == 0: drop the entry entirely
	}
	if len(out) == 0 {
		return nil
	}
	raw, _ := marshalNoEscape(out)
	return raw
}

// marshalNoEscape encodes v to JSON without escaping <, >, & as Unicode escapes.
func marshalNoEscape(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encoder appends a newline; trim it so the result is a clean JSON value.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func loadOrEmpty(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func writeAtomic(path string, top map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(top); err != nil {
		return err
	}
	data := bytes.TrimRight(buf.Bytes(), "\n")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic write failed: %w", err)
	}
	return nil
}
