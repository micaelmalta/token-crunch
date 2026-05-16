# token-crunch

A token compression engine for Claude Code that is **context-aware, structure-aware, and session-aware** — not a bag of per-command regex heuristics.

The core thesis: every tool call carries its own compression signal. The arguments the model passed *are* the query. Use that signal — plus session memory and structural analysis — to compress output intelligently, without hand-written rules per tool.

## Benchmark

```
Scenario                          Orig     Final     Saved   Ratio  Strategy
────────────────────────────────────────────────────────────────────────────────
repeated file read (×5)             1k       154       971   86.3%  dedup
repeated git status (×4)           348       150       198   56.9%  dedup
JSON array (50 items)               87        19        68   78.2%  structure
stack trace collapse                37        29         8   21.6%  structure
file tree (>10 children)           165        29       136   82.4%  structure
test output (40 pass, 2 fail)       283        48       235   83.0%  structure
log stream (repeated lines)        310        39       271   87.4%  structure
relevance trim (cat config)        13k        1k       12k   86.9%  relevance
combined session (6 turns)         946       404       542   57.3%  dedup
────────────────────────────────────────────────────────────────────────────────
TOTAL                              17k        2k       14k   84.4%
```

Run it yourself:

```bash
go run ./cmd/benchmark
```

## Install

```bash
go install github.com/micaelmalta/token-crunch/cmd/token-crunch@latest
token-crunch install
```

Tagged releases also publish prebuilt binaries for Linux, macOS, and Windows. Download the matching `token-crunch-<os>-<arch>` asset, verify its `.sha256`, put it on your `PATH`, then run `token-crunch install`.

`install` merges three hooks into `~/.claude/settings.json` non-destructively and writes atomically. Existing hooks are preserved.

## How it works

Three Claude Code hooks, one session store:

```
PreToolUse  ──► cache hit? inject stored result as context
PostToolUse ──► compress output → replace tool output → store in session cache
Stop        ──► increment turn counter → flush session store
```

State lives in `~/.local/share/token-crunch/session-<id>.json` — one file per Claude Code session, derived from `CLAUDE_SESSION_ID`. No daemon, no sidecar. Each hook invocation is a fresh process that loads and writes the store file. `PostToolUse` flushes after each successful output so the next hook process can reuse it; `Stop` advances the turn counter and flushes again.

Every compressed output gets a one-line header so the model knows what happened:

```
[token-crunch: 4,200 → 310 tokens | dedup+structure]
```

Errors always pass through unmodified. If output contains `error:`, `exception:`, `fatal:`, `panic:`, or `traceback` in the first 200 bytes, the pipeline is skipped entirely.

## Three compression strategies

### 1. Differential dedup

Tracks a content-addressed store of everything sent to the model this session.

- **Exact match** → `[unchanged — seen 2 turn(s) ago by Read]`
- **Near-match** (≥85% token similarity via LCS ratio) → sends only the unified diff

High-value cases: repeated `cat` of the same file, `git status` called multiple times, any re-read within a session. Savings: 56–86% on repeated reads.

### 2. Structural collapse

Detects output shape and applies structure-specific reduction — not regex, but shape analysis:

| Structure | Detection | Collapse rule |
|---|---|---|
| JSON array | parse attempt | show 2 examples + `…and N more` |
| JSON object | parse attempt | show first keys + remaining key count |
| NDJSON / JSONL | line-level parse | show 2 records + remaining record count |
| CSV | `encoding/csv` parse | show header + 2 rows + remaining row count |
| YAML / TOML | key/section patterns | keep initial keys or sections + summary |
| Unified diff | hunk/change markers | keep headers, first changes, and summary |
| Compiler diagnostics | `file:line` pattern | keep first diagnostics + summary |
| Coverage / package logs | summary/warning patterns | keep low-signal summaries and warnings |
| Markup | XML/HTML-like tags | show tag counts |
| Stack trace | indented frame pattern | keep header + first frame + last 3 frames |
| File tree | box-drawing / indent pattern | collapse subtrees with >10 children |
| Test output | pass/fail pattern | show only failures + summary |
| Log stream | timestamp pattern | deduplicate repeated lines with `[×N]` count |
| Tabular data | column alignment | drop columns not referenced in tool arguments |

Rules are structural, not per-command. A JSON array from `curl` and one from a database tool get the same treatment.

### 3. Relevance trimming

Uses tool arguments as a query against the output:

- Score each line by TF-IDF overlap with the query terms (no model call, ~2ms)
- Drop lines below the relevance threshold when output exceeds the token budget
- Always preserve: first line, last line, any line containing a query term verbatim

Example: `cat config/app.conf` with tool input `{"query": "database timeout"}` across a 3000-line file keeps only the relevant block. Savings: 83% on large files with sparse relevance.

## CLI

```bash
token-crunch install          # write hooks into ~/.claude/settings.json
token-crunch uninstall        # remove hooks from ~/.claude/settings.json
token-crunch pre              # PreToolUse entrypoint  (stdin → stdout)
token-crunch post             # PostToolUse entrypoint (stdin → stdout)
token-crunch flush            # Stop entrypoint        (persist session store)
token-crunch stats [--json]   # cumulative savings across all sessions
token-crunch clean [days]     # remove session files older than days (0 removes all)
token-crunch explain <file>   # explain compression decision for a hook/replay payload
token-crunch replay <log> [--json]
                              # replay a session log with current thresholds
token-crunch version          # print version
```

## Hook I/O contract

**PreToolUse** — cache miss: empty stdout (tool runs normally). Cache hit: writes `hookSpecificOutput.additionalContext` with the stored result so Claude can see the prior output before the tool executes.

**PostToolUse** — reads `{"tool_name", "tool_input", "tool_response": ...}` from stdin, compresses the extracted text, and writes `hookSpecificOutput.updatedToolOutput` when the response shape can be preserved. Unknown shapes fall back to `additionalContext`.

**Stop** — drains stdin (turn summary JSON), increments turn counter, flushes store to disk.

## Session stats

```bash
$ token-crunch stats
token-crunch stats — 3 session(s), 47 cached outputs

Session                                     Orig      Comp     Saved   Ratio
--------------------------------------------------------------------------------
session-abc123.json                        1.2MB    184.3KB   1.0MB   85.0%
session-def456.json                      412.0KB     98.1KB  313.9KB  76.2%
session-default.json                      88.3KB     31.2KB   57.1KB  64.7%
--------------------------------------------------------------------------------
TOTAL                                      1.7MB    313.6KB   1.4MB   82.1%
```

`stats --json` emits the same data as structured JSON, including per-strategy savings, per-tool savings, top saved outputs, top cached entries, session timeline, and PreToolUse cache hit/miss counts.

## Configuration

Runtime behavior can be tuned with environment variables:

| Variable | Default | Purpose |
|---|---:|---|
| `TOKEN_CRUNCH_CONFIG` | empty | Optional JSON config file path |
| `TOKEN_CRUNCH_TOKEN_BUDGET` | `2000` | Token estimate target for relevance trimming |
| `TOKEN_CRUNCH_NEAR_MATCH` | `0.85` | Similarity threshold for near-match diffs |
| `TOKEN_CRUNCH_MIN_RELEVANCE_LINES` | `10` | Minimum non-empty lines before relevance trimming |
| `TOKEN_CRUNCH_STRATEGIES` | `dedup,structure,relevance` | Comma-separated enabled strategies |
| `TOKEN_CRUNCH_ERROR_KEYWORDS` | built-in list | Comma-separated passthrough keywords |
| `TOKEN_CRUNCH_MAX_SESSION_BYTES` | `10485760` | Best-effort session file size cap |
| `TOKEN_CRUNCH_RETENTION_DAYS` | `30` | Default retention policy for cleanup |
| `TOKEN_CRUNCH_STORE_RAW` | `true` | Store raw output locally for dedup/cache; set `false` for hash/metadata-only storage |
| `TOKEN_CRUNCH_DENYLIST` | empty | Comma-separated tool/input substrings that disable raw storage |
| `TOKEN_CRUNCH_CAPTURE_DIR` | empty | Optional directory for raw hook payload capture fixtures |
| `TOKEN_CRUNCH_DEBUG` | `false` | Emit hook decision logs to stderr |

Use `token-crunch explain <payload.json>` to see which strategies were considered, which applied, and the final output that would be sent back through the hook.

Session files are local JSON files under `~/.local/share/token-crunch`. By default they contain raw tool output so dedup and cache hits work across hook processes. For sensitive projects, set `TOKEN_CRUNCH_STORE_RAW=false`, use `TOKEN_CRUNCH_DENYLIST`, and run `token-crunch clean` regularly.

To capture live Claude Code hook payloads for fixture work:

```bash
TOKEN_CRUNCH_CAPTURE_DIR=./captures token-crunch install
```

Each hook invocation writes its raw stdin payload to that directory. Review captures before committing them; they can contain local file contents, commands, paths, and secrets.

## Project layout

```
cmd/
  token-crunch/   main.go         CLI entry point
  benchmark/      main.go         savings benchmark across 9 realistic scenarios
internal/
  session/
    store.go      content-addressed session store (in-memory + atomic disk flush)
    diff.go       LCS-based diff ratio + unified diff for near-match output
  compress/
    pipeline.go   composes strategies, applies token budget cap, error passthrough
    dedup.go      strategy 1: differential dedup against session store
    structure.go  strategy 2: shape detectors (JSON/NDJSON/CSV/YAML/TOML/diff/diagnostics/coverage/packages/markup/tree/test/log/table)
    relevance.go  strategy 3: TF-IDF line scorer against tool arguments
  hook/
    pre.go        PreToolUse wiring + cache-hit context injection
    post.go       PostToolUse wiring + updatedToolOutput replacement
    flush.go      Stop hook — increments turn, flushes store
    explain.go    explain command for compression decisions
    replay.go     replay command for threshold tuning
  config/
    config.go     environment-driven runtime configuration
  install/
    install.go    non-destructive read/merge/write of ~/.claude/settings.json
scripts/
  benchmark.sh    shell benchmark: runs a Claude session with/without, diffs token counts
```

## Design constraints

1. **No model calls in the hot path.** Every hook invocation completes in <10ms. LLM-assisted compression is a non-goal.
2. **Lossless for errors.** Error signals always pass through unmodified.
3. **Transparent.** Compressed output includes a one-line header. The model sees what happened.
4. **Idempotent.** Same input → same output, always.
5. **No daemon.** State lives only in the session store file, loaded fresh each invocation.
6. **Safe installs.** `install` reads existing `settings.json`, merges non-destructively, writes atomically.

## Composability

token-crunch complements [context-broker](https://github.com/micaelmalta/context-broker):

| Project | Saves tokens on | Mechanism |
|---|---|---|
| context-broker | Tool *schema* loading | Deferred activation |
| token-crunch | Tool *output* content | Compression pipeline |

Run both — savings are additive.
