package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookEntry struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

type hooksSection struct {
	PreToolUse  []hookEntry `json:"PreToolUse,omitempty"`
	PostToolUse []hookEntry `json:"PostToolUse,omitempty"`
	Stop        []hookEntry `json:"Stop,omitempty"`
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

	// Parse as generic map to preserve unknown fields
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		top = map[string]json.RawMessage{}
	}

	// Parse hooks section
	var hooks hooksSection
	if hooksRaw, ok := top["hooks"]; ok {
		_ = json.Unmarshal(hooksRaw, &hooks)
	}

	// Merge our hooks (idempotent — check before adding)
	hooks.PreToolUse = mergeHook(hooks.PreToolUse, hookEntry{
		Matcher: ".*",
		Hooks:   []hookCommand{{Type: "command", Command: "token-crunch pre"}},
	})
	hooks.PostToolUse = mergeHook(hooks.PostToolUse, hookEntry{
		Matcher: ".*",
		Hooks:   []hookCommand{{Type: "command", Command: "token-crunch post"}},
	})
	hooks.Stop = mergeHook(hooks.Stop, hookEntry{
		Hooks: []hookCommand{{Type: "command", Command: "token-crunch flush"}},
	})

	hooksJSON, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	top["hooks"] = hooksJSON

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

	var hooks hooksSection
	if hooksRaw, ok := top["hooks"]; ok {
		_ = json.Unmarshal(hooksRaw, &hooks)
	}

	hooks.PreToolUse = removeHook(hooks.PreToolUse, "token-crunch pre")
	hooks.PostToolUse = removeHook(hooks.PostToolUse, "token-crunch post")
	hooks.Stop = removeHook(hooks.Stop, "token-crunch flush")

	hooksJSON, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	top["hooks"] = hooksJSON

	return writeAtomic(path, top)
}

func mergeHook(existing []hookEntry, entry hookEntry) []hookEntry {
	cmd := entry.Hooks[0].Command
	for _, e := range existing {
		for _, h := range e.Hooks {
			if h.Command == cmd {
				return existing // already present
			}
		}
	}
	return append(existing, entry)
}

func removeHook(existing []hookEntry, cmd string) []hookEntry {
	var out []hookEntry
	for _, e := range existing {
		var hooks []hookCommand
		for _, h := range e.Hooks {
			if h.Command != cmd {
				hooks = append(hooks, h)
			}
		}
		if len(hooks) > 0 {
			e.Hooks = hooks
			out = append(out, e)
		}
	}
	return out
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
	data, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic write failed: %w", err)
	}
	return nil
}
