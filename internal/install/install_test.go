package install

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func settingsFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", dir)
	return path
}

func readHooks(t *testing.T, path string) map[string][]hookEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	hooks := map[string][]hookEntry{}
	if raw, ok := top["hooks"]; ok {
		var hooksRaw map[string]json.RawMessage
		if err := json.Unmarshal(raw, &hooksRaw); err != nil {
			t.Fatalf("parse hooks: %v", err)
		}
		for k, v := range hooksRaw {
			var entries []hookEntry
			if err := json.Unmarshal(v, &entries); err != nil {
				t.Fatalf("parse hook entries for %s: %v", k, err)
			}
			hooks[k] = entries
		}
	}
	return hooks
}

func hasCommand(entries []hookEntry, cmd string) bool {
	for _, e := range entries {
		for _, h := range e.Hooks {
			if h.Command == cmd {
				return true
			}
		}
	}
	return false
}

func TestInstall_writesAllThreeHooks(t *testing.T) {
	path := settingsFile(t, "")

	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	hooks := readHooks(t, path)
	if !hasCommand(hooks["PreToolUse"], "token-crunch pre") {
		t.Error("PreToolUse hook missing")
	}
	if !hasCommand(hooks["PostToolUse"], "token-crunch post") {
		t.Error("PostToolUse hook missing")
	}
	if !hasCommand(hooks["Stop"], "token-crunch flush") {
		t.Error("Stop hook missing")
	}
}

func TestInstall_idempotent(t *testing.T) {
	settingsFile(t, "")

	if err := Install(false); err != nil {
		t.Fatal(err)
	}
	if err := Install(false); err != nil {
		t.Fatal(err)
	}

	hooks := readHooks(t, claudeSettingsPath(false))
	count := 0
	for _, e := range hooks["PostToolUse"] {
		for _, h := range e.Hooks {
			if h.Command == "token-crunch post" {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 post hook after double install, got %d", count)
	}
}

func TestInstall_preservesExistingHooks(t *testing.T) {
	existing := `{
		"hooks": {
			"PostToolUse": [
				{"matcher": ".*", "hooks": [{"type": "command", "command": "my-other-hook"}]}
			]
		},
		"model": "claude-opus-4"
	}`
	path := settingsFile(t, existing)

	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	hooks := readHooks(t, path)
	if !hasCommand(hooks["PostToolUse"], "my-other-hook") {
		t.Error("existing hook must be preserved after install")
	}
	if !hasCommand(hooks["PostToolUse"], "token-crunch post") {
		t.Error("new hook must be added alongside existing")
	}

	// Non-hooks fields must survive
	data, _ := os.ReadFile(path)
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("parse settings after install: %v", err)
	}
	if _, ok := top["model"]; !ok {
		t.Error("existing non-hooks fields must be preserved")
	}
}

func TestUninstall_removesHooks(t *testing.T) {
	path := settingsFile(t, "")

	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := Uninstall(false); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	hooks := readHooks(t, path)
	if hasCommand(hooks["PreToolUse"], "token-crunch pre") {
		t.Error("pre hook must be removed")
	}
	if hasCommand(hooks["PostToolUse"], "token-crunch post") {
		t.Error("post hook must be removed")
	}
	if hasCommand(hooks["Stop"], "token-crunch flush") {
		t.Error("flush hook must be removed")
	}
}

func TestUninstall_preservesOtherHooks(t *testing.T) {
	existing := `{"hooks":{"PostToolUse":[
		{"matcher":".*","hooks":[{"type":"command","command":"keep-me"}]},
		{"matcher":".*","hooks":[{"type":"command","command":"token-crunch post"}]}
	]}}`
	path := settingsFile(t, existing)

	if err := Uninstall(false); err != nil {
		t.Fatal(err)
	}

	hooks := readHooks(t, path)
	if !hasCommand(hooks["PostToolUse"], "keep-me") {
		t.Error("unrelated hook must survive uninstall")
	}
	if hasCommand(hooks["PostToolUse"], "token-crunch post") {
		t.Error("token-crunch hook must be removed")
	}
}

func TestUninstall_missingFile(t *testing.T) {
	settingsFile(t, "") // sets HOME but writes no file
	// Uninstall on a non-existent settings file must not error
	if err := Uninstall(false); err != nil {
		t.Fatalf("Uninstall on missing file must not error: %v", err)
	}
}

func TestUninstall_emptyEventTypeIsRemoved(t *testing.T) {
	// After removing the only hook in an event type, the key must not remain
	// as a null value in the JSON output.
	path := settingsFile(t, "")
	if err := Install(false); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("null")) {
		t.Errorf("output must not contain null values for emptied event types, got:\n%s", data)
	}
}

func TestInstall_preservesUnknownFieldsOnExistingEntries(t *testing.T) {
	// Entries may carry extra fields like "if" that our schema doesn't know about.
	// Install must not drop them when it appends its own entry to the same event type.
	existing := `{"hooks":{"PreToolUse":[{"if":"Bash(git worktree add *)","matcher":".*","hooks":[{"type":"command","command":"my-guard"}]}]}}`
	path := settingsFile(t, existing)

	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"if"`)) {
		t.Error(`existing "if" field must be preserved after install`)
	}
	if !bytes.Contains(data, []byte(`"my-guard"`)) {
		t.Error("existing hook command must be preserved after install")
	}
	hooks := readHooks(t, path)
	if !hasCommand(hooks["PreToolUse"], "token-crunch pre") {
		t.Error("new token-crunch pre hook must be added")
	}
}

func TestInstall_preservesUnknownEventTypes(t *testing.T) {
	existing := `{
		"hooks": {
			"Notification":      [{"hooks": [{"type": "command", "command": "notify-me"}]}],
			"UserPromptSubmit":  [{"hooks": [{"type": "command", "command": "on-submit"}]}],
			"WorktreeCreate":    [{"hooks": [{"type": "command", "command": "on-worktree"}]}]
		}
	}`
	path := settingsFile(t, existing)

	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	hooks := readHooks(t, path)
	if !hasCommand(hooks["Notification"], "notify-me") {
		t.Error("Notification hook must be preserved")
	}
	if !hasCommand(hooks["UserPromptSubmit"], "on-submit") {
		t.Error("UserPromptSubmit hook must be preserved")
	}
	if !hasCommand(hooks["WorktreeCreate"], "on-worktree") {
		t.Error("WorktreeCreate hook must be preserved")
	}
}

func TestInstall_noHTMLEscaping(t *testing.T) {
	existing := `{"hooks":{"PreToolUse":[{"matcher":".*","hooks":[{"type":"command","command":"echo a >> /tmp/log"}]}]}}`
	path := settingsFile(t, existing)

	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("\\u003e")) {
		t.Error("output must not contain Unicode-escaped '>' (\\u003e) characters")
	}
}

func TestInstall_atomicWrite(t *testing.T) {
	path := settingsFile(t, "")
	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	// .tmp must not linger
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal(".tmp file must be cleaned up after install")
	}
}

func TestInstall_local_writesProjectSettings(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := Install(true); err != nil {
		t.Fatalf("Install(local): %v", err)
	}

	localPath := filepath.Join(dir, localSettingsPath)
	hooks := readHooks(t, localPath)
	if !hasCommand(hooks["PreToolUse"], "token-crunch pre") {
		t.Error("PreToolUse hook missing in local settings")
	}
	if !hasCommand(hooks["PostToolUse"], "token-crunch post") {
		t.Error("PostToolUse hook missing in local settings")
	}
	if !hasCommand(hooks["Stop"], "token-crunch flush") {
		t.Error("Stop hook missing in local settings")
	}

	// Global settings must be untouched (HOME was set to an empty temp dir)
	globalPath := claudeSettingsPath(false)
	if _, err := os.Stat(globalPath); !os.IsNotExist(err) {
		t.Error("global settings must not be written during local install")
	}
}

func TestUninstall_local_removesProjectSettings(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := Install(true); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(dir, localSettingsPath)

	if err := Uninstall(true); err != nil {
		t.Fatalf("Uninstall(local): %v", err)
	}

	hooks := readHooks(t, localPath)
	if hasCommand(hooks["PreToolUse"], "token-crunch pre") {
		t.Error("pre hook must be removed from local settings")
	}
	if hasCommand(hooks["PostToolUse"], "token-crunch post") {
		t.Error("post hook must be removed from local settings")
	}
	if hasCommand(hooks["Stop"], "token-crunch flush") {
		t.Error("flush hook must be removed from local settings")
	}
}
