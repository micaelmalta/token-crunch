# Session Building

Subdomain: session/
Source paths: internal/session/

Content-addressed in-memory session store with atomic disk persistence. Each hook process restores state from a per-session JSON file on init and flushes it on the Stop hook. The diff utilities support the near-match dedup path in compress/dedup.go.

## TASK → LOAD

| Task | Load |
|------|------|
| Understand session persistence and cross-process state | store.md |
| Debug cache miss or incorrect dedup detection | store.md |
| Change similarity threshold for near-match dedup | store.md |
| Add new fields to the session entry format | store.md |
| Understand how unified diffs are produced | store.md |
| Change the DP/greedy edit-script fallback threshold | store.md |
| Add new per-session metrics | store.md |

## Rooms

| Room | Source paths | Files |
|------|-------------|-------|
| store.md | internal/session/ | store.go, diff.go |
