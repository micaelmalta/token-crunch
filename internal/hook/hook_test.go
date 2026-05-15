package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmalta/token-crunch/internal/session"
)

// ── toolText ─────────────────────────────────────────────────────────────────

func TestToolText_bash_stdout(t *testing.T) {
	raw := json.RawMessage(`{"stdout":"hello world","stderr":""}`)
	if got := toolText(raw); got != "hello world" {
		t.Fatalf("want 'hello world', got %q", got)
	}
}

func TestToolText_bash_empty_stdout(t *testing.T) {
	raw := json.RawMessage(`{"stdout":"","stderr":"some warning"}`)
	if got := toolText(raw); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestToolText_content_key(t *testing.T) {
	raw := json.RawMessage(`{"content":"file contents here"}`)
	if got := toolText(raw); got != "file contents here" {
		t.Fatalf("want 'file contents here', got %q", got)
	}
}

func TestToolText_text_key(t *testing.T) {
	raw := json.RawMessage(`{"text":"some text"}`)
	if got := toolText(raw); got != "some text" {
		t.Fatalf("want 'some text', got %q", got)
	}
}

func TestToolText_output_key(t *testing.T) {
	raw := json.RawMessage(`{"output":"generic output"}`)
	if got := toolText(raw); got != "generic output" {
		t.Fatalf("want 'generic output', got %q", got)
	}
}

func TestToolText_mcp_content_array(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"image","text":""},{"type":"text","text":"mcp output"}]}`)
	if got := toolText(raw); got != "mcp output" {
		t.Fatalf("want 'mcp output', got %q", got)
	}
}

func TestToolText_mcp_content_array_joinsMultipleTextBlocks(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"text","text":"first"},{"type":"image","source":"x"},{"type":"text","text":"second"}]}`)
	if got := toolText(raw); got != "first\n\nsecond" {
		t.Fatalf("want joined text blocks, got %q", got)
	}
}

func TestToolText_nestedContent(t *testing.T) {
	raw := json.RawMessage(`{"result":{"content":[{"type":"text","text":"nested"}]}}`)
	if got := toolText(raw); got != "nested" {
		t.Fatalf("want nested text, got %q", got)
	}
}

func TestToolText_binaryAndHugeLineSkipped(t *testing.T) {
	if got := toolText(json.RawMessage("{\"content\":\"abc\\u0000def\"}")); got != "" {
		t.Fatalf("binary-looking content must be skipped, got %q", got)
	}
	huge := strings.Repeat("a", 21000)
	raw, _ := json.Marshal(map[string]string{"content": huge})
	if got := toolText(raw); got != "" {
		t.Fatalf("huge single-line content must be skipped, got len %d", len(got))
	}
}

func TestToolText_mcp_content_array_skips_empty(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"text","text":""},{"type":"text","text":"second"}]}`)
	if got := toolText(raw); got != "second" {
		t.Fatalf("want 'second', got %q", got)
	}
}

func TestToolText_invalid_json(t *testing.T) {
	raw := json.RawMessage(`not json`)
	if got := toolText(raw); got != "" {
		t.Fatalf("invalid JSON must return empty, got %q", got)
	}
}

func TestToolText_no_known_key(t *testing.T) {
	raw := json.RawMessage(`{"result":42}`)
	if got := toolText(raw); got != "" {
		t.Fatalf("unknown key must return empty, got %q", got)
	}
}

// ── isError ───────────────────────────────────────────────────────────────────

func TestIsError_isError_true(t *testing.T) {
	raw := json.RawMessage(`{"isError":true,"content":"oops"}`)
	if !isError(raw) {
		t.Fatal("isError:true must be detected as error")
	}
}

func TestIsError_isError_false(t *testing.T) {
	raw := json.RawMessage(`{"isError":false,"content":"ok"}`)
	if isError(raw) {
		t.Fatal("isError:false must not be detected as error")
	}
}

func TestIsError_bash_stderr_only(t *testing.T) {
	raw := json.RawMessage(`{"stdout":"","stderr":"command not found"}`)
	if !isError(raw) {
		t.Fatal("non-empty stderr with empty stdout must be treated as error")
	}
}

func TestIsError_bash_both_stdout_stderr(t *testing.T) {
	// stderr alongside real stdout is a warning, not an error
	raw := json.RawMessage(`{"stdout":"output","stderr":"warning"}`)
	if isError(raw) {
		t.Fatal("non-empty stdout alongside stderr must not be treated as error")
	}
}

func TestIsError_no_error_fields(t *testing.T) {
	raw := json.RawMessage(`{"content":"all good"}`)
	if isError(raw) {
		t.Fatal("response with no error fields must not be an error")
	}
}

func TestIsError_invalid_json(t *testing.T) {
	if isError(json.RawMessage(`bad`)) {
		t.Fatal("invalid JSON must not be treated as error")
	}
}

// ── runPost (I/O-decoupled helper) ───────────────────────────────────────────

// runPost exercises the Post logic with injected stdin/stdout.
func runPost(t *testing.T, _ *session.Store, inputJSON string) string {
	t.Helper()

	// Capture stdout via a pipe
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Feed stdin
	origStdin := os.Stdin
	sr, sw, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	os.Stdin = sr
	if _, err := sw.WriteString(inputJSON); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	sw.Close()

	postErr := Post()

	w.Close()
	os.Stdout = origStdout
	os.Stdin = origStdin
	sr.Close()

	if postErr != nil {
		t.Fatalf("Post() error: %v", postErr)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, _ := r.Read(buf)
		if n == 0 {
			break
		}
		sb.Write(buf[:n])
	}
	r.Close()
	return sb.String()
}

func TestPost_emptyContent(t *testing.T) {
	// stdout="" — nothing to compress, no output expected
	input := `{"session_id":"test","tool_name":"Bash","tool_input":{},"tool_response":{"stdout":"","stderr":""}}`
	out := runPost(t, session.NewEphemeral(), input)
	if out != "" {
		t.Fatalf("expected no output for empty content, got: %s", out)
	}
}

func TestPost_errorPassthrough(t *testing.T) {
	input := `{"session_id":"test","tool_name":"Bash","tool_input":{},"tool_response":{"isError":true,"content":"oops"}}`
	out := runPost(t, session.NewEphemeral(), input)
	if out != "" {
		t.Fatalf("error responses must produce no output, got: %s", out)
	}
}

func TestPost_invalidJSON(t *testing.T) {
	out := runPost(t, session.NewEphemeral(), `not json`)
	if out != "" {
		t.Fatalf("invalid JSON must produce no output, got: %s", out)
	}
}

func TestPost_compressedOutputFormat(t *testing.T) {
	// Large JSON array — structure strategy fires, output must contain hookSpecificOutput
	items := make([]string, 10)
	for i := range items {
		items[i] = `{"id":` + string(rune('0'+i)) + `,"name":"user"}`
	}
	content := "[" + strings.Join(items, ",") + "]"
	resp, _ := json.Marshal(map[string]string{"content": content})
	input, _ := json.Marshal(map[string]any{
		"session_id":    "test-compress",
		"tool_name":     "Bash",
		"tool_input":    map[string]any{"command": "curl /api/users"},
		"tool_response": json.RawMessage(resp),
	})

	out := runPost(t, session.NewEphemeral(), string(input))
	if out == "" {
		t.Fatal("expected compressed output for large JSON array")
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("output must be valid JSON: %v\nout: %s", err, out)
	}
	hso, ok := env["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("expected hookSpecificOutput key, got: %v", env)
	}
	if hso["hookEventName"] != "PostToolUse" {
		t.Fatalf("expected hookEventName=PostToolUse, got: %v", hso["hookEventName"])
	}
	updated, ok := hso["updatedToolOutput"].(map[string]any)
	if !ok {
		t.Fatalf("expected updatedToolOutput key, got: %v", hso)
	}
	updatedContent, _ := updated["content"].(string)
	if !strings.HasPrefix(updatedContent, "[token-crunch:") {
		t.Fatalf("updated content must start with token-crunch header, got: %s", updatedContent)
	}
}

func TestUpdatedToolOutput_bashStdout(t *testing.T) {
	raw := json.RawMessage(`{"stdout":"original","stderr":"warning","interrupted":false}`)
	updated, ok := updatedToolOutput(raw, "replacement")
	if !ok {
		t.Fatal("expected Bash output to be updateable")
	}
	m := updated.(map[string]any)
	if m["stdout"] != "replacement" {
		t.Fatalf("stdout must be replaced, got: %v", m["stdout"])
	}
	if m["stderr"] != "warning" {
		t.Fatalf("stderr must be preserved, got: %v", m["stderr"])
	}
	if m["interrupted"] != false {
		t.Fatalf("other fields must be preserved, got: %v", m["interrupted"])
	}
}

func TestUpdatedToolOutput_mcpContentArray(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"image","source":"x"},{"type":"text","text":"original"}],"isError":false}`)
	updated, ok := updatedToolOutput(raw, "replacement")
	if !ok {
		t.Fatal("expected MCP content array to be updateable")
	}
	m := updated.(map[string]any)
	items := m["content"].([]any)
	textItem := items[1].(map[string]any)
	if textItem["text"] != "replacement" {
		t.Fatalf("text content must be replaced, got: %v", textItem["text"])
	}
	if items[0].(map[string]any)["type"] != "image" {
		t.Fatalf("non-text content must be preserved, got: %v", items[0])
	}
}

func TestUpdatedToolOutput_mcpContentArray_collapsesTextBlocks(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"text","text":"first"},{"type":"image","source":"x"},{"type":"text","text":"second"}]}`)
	updated, ok := updatedToolOutput(raw, "replacement")
	if !ok {
		t.Fatal("expected MCP content array to be updateable")
	}
	m := updated.(map[string]any)
	items := m["content"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected one text block plus image, got %d items: %+v", len(items), items)
	}
	if items[0].(map[string]any)["text"] != "replacement" {
		t.Fatalf("first text block must be replacement, got: %+v", items[0])
	}
	if items[1].(map[string]any)["type"] != "image" {
		t.Fatalf("non-text block must be preserved, got: %+v", items[1])
	}
}

func TestUpdatedToolOutput_unknownShape(t *testing.T) {
	raw := json.RawMessage(`{"output":"original"}`)
	if updated, ok := updatedToolOutput(raw, "replacement"); ok {
		t.Fatalf("generic output shape must fall back to additionalContext, got: %v", updated)
	}
}

func TestHookGoldenFixtures(t *testing.T) {
	cases := []struct {
		name string
		kind string
	}{
		{name: "post_bash_stdout", kind: "post"},
		{name: "post_content_string", kind: "post"},
		{name: "post_text_string", kind: "post"},
		{name: "post_mcp_content_array", kind: "post"},
		{name: "post_unknown_shape", kind: "post"},
		{name: "pre_cache_hit", kind: "pre"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := readFixture(t, tc.name+".input.json")
			var out bytes.Buffer
			store := session.NewEphemeral()

			switch tc.kind {
			case "post":
				var inp postInput
				if err := json.Unmarshal(input, &inp); err != nil {
					t.Fatalf("parse post fixture: %v", err)
				}
				if err := postWithStore(store, inp, &out, false); err != nil {
					t.Fatalf("postWithStore: %v", err)
				}
			case "pre":
				var inp preInput
				if err := json.Unmarshal(input, &inp); err != nil {
					t.Fatalf("parse pre fixture: %v", err)
				}
				const cachedContent = "cached result from prior PostToolUse"
				cacheKey := buildCacheKey(inp.ToolName, inp.ToolInput)
				store.PutToolCache(cacheKey, inp.ToolName, cachedContent, len(cachedContent))
				if err := preWithStore(store, inp, &out); err != nil {
					t.Fatalf("preWithStore: %v", err)
				}
			default:
				t.Fatalf("unknown fixture kind %q", tc.kind)
			}

			want := readFixture(t, tc.name+".golden.json")
			assertJSONEqual(t, want, out.Bytes())
		})
	}
}

func TestExplain_postPayload(t *testing.T) {
	payload := readFixture(t, "post_content_string.input.json")
	path := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureHookStdout(t, func() {
		if err := Explain(path); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})
	var report ExplainReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("explain output must be JSON: %v\n%s", err, out)
	}
	if !report.Modified || !strings.Contains(strings.Join(report.Strategies, ","), "structure") {
		t.Fatalf("expected structure explanation, got %+v", report)
	}
}

func TestCapturePayload_writesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TOKEN_CRUNCH_CAPTURE_DIR", dir)
	capturePayload("PostToolUse", []byte(`{"ok":true}`))

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want one capture file, got %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("unexpected capture payload: %s", data)
	}
}

func captureHookStdout(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = orig

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, _ := r.Read(buf)
		if n == 0 {
			break
		}
		sb.Write(buf[:n])
	}
	r.Close()
	return sb.String()
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func assertJSONEqual(t *testing.T, want, got []byte) {
	t.Helper()
	wantNorm := normaliseJSON(t, want)
	gotNorm := normaliseJSON(t, got)
	if string(gotNorm) != string(wantNorm) {
		t.Fatalf("JSON mismatch\nwant:\n%s\n\ngot:\n%s", wantNorm, gotNorm)
	}
}

func normaliseJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("invalid JSON:\n%s\nerror: %v", data, err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("normalise JSON: %v", err)
	}
	return out
}

// ── Flush ────────────────────────────────────────────────────────────────────

func runFlush(t *testing.T, inputJSON string) error {
	t.Helper()
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	if _, err := w.WriteString(inputJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()
	flushErr := Flush()
	os.Stdin = origStdin
	r.Close()
	return flushErr
}

func TestFlush_noError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	// Flush must not error regardless of whether the global store was already
	// initialised in this process to a different session ID.
	if err := runFlush(t, `{"session_id":"flush-noerror"}`); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestFlush_emptyInput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	// Empty JSON — session_id falls back to "default"
	if err := runFlush(t, `{}`); err != nil {
		t.Fatalf("Flush with empty input: %v", err)
	}
}

// ── Replay ───────────────────────────────────────────────────────────────────

func writeReplayLog(t *testing.T, entries []ReplayEntry) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "replay-*.log")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, _ := json.Marshal(e)
		f.Write(b)
		f.WriteString("\n")
	}
	f.Close()
	return f.Name()
}

func TestReplay_singleEntry(t *testing.T) {
	log := writeReplayLog(t, []ReplayEntry{
		{ToolName: "Bash", ToolInput: map[string]any{"command": "git status"}, Content: "On branch main\nnothing to commit"},
	})
	if err := Replay(log); err != nil {
		t.Fatalf("Replay: %v", err)
	}
}

func TestReplayJSON(t *testing.T) {
	log := writeReplayLog(t, []ReplayEntry{
		{ToolName: "Bash", ToolInput: map[string]any{"command": "git status"}, Content: strings.Repeat("git status line\n", 30)},
	})
	out := captureHookStdout(t, func() {
		if err := ReplayJSON(log); err != nil {
			t.Fatalf("ReplayJSON: %v", err)
		}
	})
	var report ReplayReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("replay JSON must parse: %v\n%s", err, out)
	}
	if report.Total.Entries != 1 || len(report.Entries) != 1 {
		t.Fatalf("unexpected replay report: %+v", report)
	}
}

func TestReplay_skipsComments(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "replay-*.log")
	f.WriteString("# this is a comment\n")
	f.WriteString("\n")
	entry := ReplayEntry{ToolName: "Bash", Content: "output"}
	b, _ := json.Marshal(entry)
	f.Write(b)
	f.WriteString("\n")
	f.Close()

	if err := Replay(f.Name()); err != nil {
		t.Fatalf("Replay with comments: %v", err)
	}
}

func TestReplay_skipsMalformedLines(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "replay-*.log")
	f.WriteString("not json at all\n")
	entry := ReplayEntry{ToolName: "Read", Content: "valid content"}
	b, _ := json.Marshal(entry)
	f.Write(b)
	f.WriteString("\n")
	f.Close()

	// Should not error — malformed lines are skipped with a stderr warning
	if err := Replay(f.Name()); err != nil {
		t.Fatalf("Replay with malformed line: %v", err)
	}
}

func TestReplay_missingFile(t *testing.T) {
	err := Replay("/nonexistent/path/replay.log")
	if err == nil {
		t.Fatal("Replay on missing file must return an error")
	}
}

func TestReplay_dedupsAcrossTurns(t *testing.T) {
	content := strings.Repeat("git status line\n", 30)
	log := writeReplayLog(t, []ReplayEntry{
		{ToolName: "Bash", ToolInput: map[string]any{"command": "git status"}, Content: content},
		{ToolName: "Bash", ToolInput: map[string]any{"command": "git status"}, Content: content},
	})
	// Second entry should be deduped — just verify no error and it runs to completion
	if err := Replay(log); err != nil {
		t.Fatalf("Replay dedup session: %v", err)
	}
}

func TestReplay_emptyLog(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "replay-*.log")
	f.Close()
	if err := Replay(f.Name()); err != nil {
		t.Fatalf("empty log must not error: %v", err)
	}
}

// ── buildCacheKey ─────────────────────────────────────────────────────────────

func TestBuildCacheKey_deterministic(t *testing.T) {
	input := map[string]any{"command": "git status", "cwd": "/repo"}
	k1 := buildCacheKey("Bash", input)
	k2 := buildCacheKey("Bash", input)
	if k1 != k2 {
		t.Fatal("buildCacheKey must be deterministic for the same inputs")
	}
}

func TestBuildCacheKey_differentTools(t *testing.T) {
	input := map[string]any{"command": "ls"}
	if buildCacheKey("Bash", input) == buildCacheKey("Read", input) {
		t.Fatal("different tool names must produce different cache keys")
	}
}

func TestBuildCacheKey_differentInputs(t *testing.T) {
	if buildCacheKey("Bash", map[string]any{"command": "ls"}) ==
		buildCacheKey("Bash", map[string]any{"command": "pwd"}) {
		t.Fatal("different tool inputs must produce different cache keys")
	}
}

func TestBuildCacheKey_nilInput(t *testing.T) {
	// must not panic
	_ = buildCacheKey("Read", nil)
}

// ── preWithStore ──────────────────────────────────────────────────────────────

func TestPre_cacheMiss(t *testing.T) {
	store := session.NewEphemeral()
	inp := preInput{ToolName: "Bash", ToolInput: map[string]any{"command": "git status"}}
	var buf bytes.Buffer
	if err := preWithStore(store, inp, &buf); err != nil {
		t.Fatalf("preWithStore: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("cache miss must produce no output, got: %s", buf.String())
	}
}

func TestPre_cacheHit(t *testing.T) {
	store := session.NewEphemeral()
	inp := preInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "git status"},
	}
	cacheKey := buildCacheKey(inp.ToolName, inp.ToolInput)
	store.PutToolCache(cacheKey, inp.ToolName, "cached git status output", len("cached git status output"))

	var buf bytes.Buffer
	if err := preWithStore(store, inp, &buf); err != nil {
		t.Fatalf("preWithStore: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("cache hit must produce output")
	}
	var env map[string]any
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("output must be valid JSON: %v\ngot: %s", err, buf.String())
	}
	hso, ok := env["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("expected hookSpecificOutput, got: %v", env)
	}
	if hso["hookEventName"] != "PreToolUse" {
		t.Fatalf("expected hookEventName=PreToolUse, got: %v", hso["hookEventName"])
	}
	ctx, _ := hso["additionalContext"].(string)
	if ctx == "" {
		t.Fatal("additionalContext must be non-empty on cache hit")
	}
}

func TestPre_cacheHit_returnsStoredContent(t *testing.T) {
	store := session.NewEphemeral()
	inp := preInput{
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": "src/main.go"},
	}
	cacheKey := buildCacheKey(inp.ToolName, inp.ToolInput)
	const cachedContent = "package main\n\nfunc main() {}"
	store.PutToolCache(cacheKey, inp.ToolName, cachedContent, len(cachedContent))

	var buf bytes.Buffer
	if err := preWithStore(store, inp, &buf); err != nil {
		t.Fatalf("preWithStore: %v", err)
	}

	var env map[string]any
	json.Unmarshal(buf.Bytes(), &env)
	hso := env["hookSpecificOutput"].(map[string]any)
	ctx := hso["additionalContext"].(string)
	if ctx != cachedContent {
		t.Fatalf("additionalContext must equal stored content, got: %q", ctx)
	}
}

func TestPost_populatesPreToolCache(t *testing.T) {
	store := session.NewEphemeral()
	inp := postInput{
		SessionID: "cache-flow",
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": "src/main.go"},
		ToolResponse: json.RawMessage(`{
			"content": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
		}`),
	}

	var postOut bytes.Buffer
	if err := postWithStore(store, inp, &postOut, false); err != nil {
		t.Fatalf("postWithStore: %v", err)
	}
	if postOut.Len() != 0 {
		t.Fatalf("unmodified content should not emit PostToolUse output, got: %s", postOut.String())
	}

	var preOut bytes.Buffer
	preInp := preInput{ToolName: inp.ToolName, ToolInput: inp.ToolInput}
	if err := preWithStore(store, preInp, &preOut); err != nil {
		t.Fatalf("preWithStore: %v", err)
	}
	if preOut.Len() == 0 {
		t.Fatal("expected PreToolUse cache hit after PostToolUse stores result")
	}

	var env map[string]any
	if err := json.Unmarshal(preOut.Bytes(), &env); err != nil {
		t.Fatalf("pre output must be valid JSON: %v\ngot: %s", err, preOut.String())
	}
	hso := env["hookSpecificOutput"].(map[string]any)
	ctx := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "package main") {
		t.Fatalf("cached context must contain prior tool result, got: %q", ctx)
	}
}

func TestPost_populatesPreToolCacheWithCompressedResult(t *testing.T) {
	store := session.NewEphemeral()
	items := make([]string, 10)
	for i := range items {
		items[i] = `{"id":` + string(rune('0'+i)) + `,"name":"user"}`
	}
	content := "[" + strings.Join(items, ",") + "]"
	resp, _ := json.Marshal(map[string]string{"content": content})
	inp := postInput{
		SessionID:    "cache-flow-compressed",
		ToolName:     "Bash",
		ToolInput:    map[string]any{"command": "curl /api/users"},
		ToolResponse: json.RawMessage(resp),
	}

	var postOut bytes.Buffer
	if err := postWithStore(store, inp, &postOut, false); err != nil {
		t.Fatalf("postWithStore: %v", err)
	}

	var preOut bytes.Buffer
	preInp := preInput{ToolName: inp.ToolName, ToolInput: inp.ToolInput}
	if err := preWithStore(store, preInp, &preOut); err != nil {
		t.Fatalf("preWithStore: %v", err)
	}

	var env map[string]any
	if err := json.Unmarshal(preOut.Bytes(), &env); err != nil {
		t.Fatalf("pre output must be valid JSON: %v\ngot: %s", err, preOut.String())
	}
	hso := env["hookSpecificOutput"].(map[string]any)
	ctx := hso["additionalContext"].(string)
	if !strings.HasPrefix(ctx, "[token-crunch:") {
		t.Fatalf("cached compressed context must include token-crunch header, got: %s", ctx)
	}
}
