#!/usr/bin/env bash
set -euo pipefail

prompt="Return exactly: fya-live-parity-ok"
fya_bin="${FYA_BIN:-./.bin/fya}"
claude_bin="${CLAUDE_BIN:-claude}"
dry_run=0

usage() {
    cat <<'EOF'
compare-claude-p.sh compares real claude -p stream-json output with fya.

Usage:
  scripts/compare-claude-p.sh --dry-run
  scripts/compare-claude-p.sh [--prompt TEXT] [--fya PATH] [--claude PATH]

Options:
  --dry-run      validate local parsing logic without calling Claude
  --prompt TEXT  prompt used for the live parity check
  --fya PATH     fya binary path, default ./.bin/fya or FYA_BIN
  --claude PATH  claude binary path, default claude or CLAUDE_BIN
  -h, --help     show this help

Live mode consumes Claude quota twice: once through claude -p and once through fya.
EOF
}

while (($#)); do
    case "$1" in
        --dry-run)
            dry_run=1
            shift
            ;;
        --prompt)
            prompt="${2:?--prompt requires a value}"
            shift 2
            ;;
        --fya)
            fya_bin="${2:?--fya requires a value}"
            shift 2
            ;;
        --claude)
            claude_bin="${2:?--claude requires a value}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "required command not found: $1" >&2
        exit 2
    fi
}

validate_stream() {
    local name="$1"
    local file="$2"

    if [[ ! -s "$file" ]]; then
        echo "$name: stream is empty" >&2
        return 1
    fi
    if ! jq -e . "$file" >/dev/null; then
        echo "$name: stream contains invalid JSONL" >&2
        return 1
    fi

    local deltas
    deltas=$(jq -r 'select(.type == "content_block_delta" and .delta.type == "text_delta") | .delta.text' "$file" | wc -l | tr -d ' ')
    if [[ "$deltas" == "0" ]]; then
        echo "$name: no content_block_delta text events found" >&2
        return 1
    fi

    local results
    results=$(jq -r 'select(.type == "result") | .type' "$file" | wc -l | tr -d ' ')
    if [[ "$results" == "0" ]]; then
        echo "$name: no final result event found" >&2
        return 1
    fi
}

collect_text() {
    jq -rs '[.[] | select(.type == "content_block_delta" and .delta.type == "text_delta") | .delta.text] | join("")' "$1"
}

result_fields() {
    jq -sc '
        [.[] | select(.type == "result")]
        | last
        | {
            subtype,
            is_error,
            result_type: (.result | type),
            session_id,
            num_turns,
            terminal_reason
        }
    ' "$1"
}

run_dry_check() {
    local dir="$1"
    local sample="$dir/sample.jsonl"
    cat > "$sample" <<'EOF'
{"type":"content_block_delta","delta":{"type":"text_delta","text":"fya-live-parity-ok"}}
{"type":"result","subtype":"success","is_error":false,"result":"","session_id":"dry","num_turns":1,"terminal_reason":"end_turn"}
EOF
    validate_stream "dry-run" "$sample"
    echo "dry-run text: $(collect_text "$sample")"
    echo "dry-run result fields: $(result_fields "$sample")"
}

require_cmd jq

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

if [[ "$dry_run" == "1" ]]; then
    run_dry_check "$tmpdir"
    echo "dry-run ok: no Claude command was executed"
    exit 0
fi

if [[ ! -x "$fya_bin" ]]; then
    echo "fya binary is not executable: $fya_bin" >&2
    echo "run: make build" >&2
    exit 2
fi
require_cmd "$claude_bin"

claude_out="$tmpdir/claude.jsonl"
fya_out="$tmpdir/fya.jsonl"

printf '%s\n' "$prompt" | "$claude_bin" --dangerously-skip-permissions --output-format stream-json --verbose --print > "$claude_out"
printf '%s\n' "$prompt" | "$fya_bin" --dangerously-skip-permissions --output-format stream-json --verbose --print > "$fya_out"

validate_stream "claude -p" "$claude_out"
validate_stream "fya" "$fya_out"

claude_text=$(collect_text "$claude_out")
fya_text=$(collect_text "$fya_out")

echo "claude text: $claude_text"
echo "fya text:    $fya_text"
echo "claude result fields: $(result_fields "$claude_out")"
echo "fya result fields:    $(result_fields "$fya_out")"

if [[ "$claude_text" == "$fya_text" ]]; then
    echo "text comparison: exact match"
else
    echo "text comparison: differs"
fi
