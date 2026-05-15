#!/usr/bin/env bash
# Benchmark token-crunch savings on a fixed Claude Code task.
# Usage: ./scripts/benchmark.sh [task-prompt]
# Requires: claude CLI with --output-format json support.

set -euo pipefail

TASK="${1:-"List files, read package.json, run tests, read package.json again"}"
BINARY="${BINARY:-token-crunch}"
TMPDIR_BASE="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_BASE"' EXIT

log() { echo "[benchmark] $*" >&2; }

run_session() {
    local label="$1"
    local use_hooks="$2"
    local out_file="$TMPDIR_BASE/${label}.json"

    local hook_flag=""
    if [[ "$use_hooks" == "false" ]]; then
        # Temporarily disable hooks by overriding settings path
        hook_flag="--no-hooks"
    fi

    log "Running session: $label"
    # claude --output-format json outputs session metadata including token counts
    claude $hook_flag --print "$TASK" --output-format json > "$out_file" 2>/dev/null || true
    echo "$out_file"
}

extract_tokens() {
    local file="$1"
    python3 -c "
import json, sys
data = json.load(open('$file'))
# Sum all input_tokens across turns
total = sum(t.get('usage', {}).get('input_tokens', 0) for t in data.get('turns', []))
print(total)
" 2>/dev/null || echo "0"
}

log "=== token-crunch benchmark ==="
log "Task: $TASK"
echo

# Run without token-crunch
without_file="$(run_session without false)"
without_tokens="$(extract_tokens "$without_file")"
log "Without token-crunch: $without_tokens input tokens"

# Install token-crunch hooks for this run
"$BINARY" install

# Run with token-crunch
with_file="$(run_session with true)"
with_tokens="$(extract_tokens "$with_file")"
log "With token-crunch:    $with_tokens input tokens"

# Remove hooks after benchmark
"$BINARY" uninstall

# Summary
if [[ "$without_tokens" -gt 0 ]]; then
    saved=$((without_tokens - with_tokens))
    pct=$(python3 -c "print(f'{$saved/$without_tokens*100:.1f}')")
    echo
    echo "=== Results ==="
    echo "Without : $without_tokens tokens"
    echo "With    : $with_tokens tokens"
    echo "Saved   : $saved tokens ($pct%)"
else
    echo "Could not extract token counts from claude output."
fi
