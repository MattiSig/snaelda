#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMPT_FILE=""

CODEX_MODEL="${CODEX_MODEL:-gpt-5.4}"
CODEX_EXEC_MODE_ARGS_DEFAULT=("--dangerously-bypass-approvals-and-sandbox")
if [[ -n "${RALPH_CODEX_EXEC_MODE_ARGS:-}" ]]; then
  read -r -a CODEX_EXEC_MODE_ARGS <<< "$RALPH_CODEX_EXEC_MODE_ARGS"
else
  CODEX_EXEC_MODE_ARGS=("${CODEX_EXEC_MODE_ARGS_DEFAULT[@]}")
fi

CODEX_CMD_DEFAULT=("codex" "exec")
if [[ -n "${CODEX_CMD:-}" ]]; then
  read -r -a CODEX_CMD_ARR <<< "$CODEX_CMD"
else
  CODEX_CMD_ARR=("${CODEX_CMD_DEFAULT[@]}")
fi

# Claude model is picked interactively unless CLAUDE_MODEL is set.
# Override the picker list with RALPH_CLAUDE_MODELS (space-separated model ids).
CLAUDE_MODELS_DEFAULT=("claude-fable-5" "claude-opus-4-8" "claude-sonnet-4-6" "claude-haiku-4-5-20251001")
if [[ -n "${RALPH_CLAUDE_MODELS:-}" ]]; then
  read -r -a CLAUDE_MODELS <<< "$RALPH_CLAUDE_MODELS"
else
  CLAUDE_MODELS=("${CLAUDE_MODELS_DEFAULT[@]}")
fi
CLAUDE_EXEC_MODE_ARGS_DEFAULT=("--dangerously-skip-permissions" "--verbose" "--output-format" "stream-json")
if [[ -n "${RALPH_CLAUDE_EXEC_MODE_ARGS:-}" ]]; then
  read -r -a CLAUDE_EXEC_MODE_ARGS <<< "$RALPH_CLAUDE_EXEC_MODE_ARGS"
else
  CLAUDE_EXEC_MODE_ARGS=("${CLAUDE_EXEC_MODE_ARGS_DEFAULT[@]}")
fi

CLAUDE_CMD_DEFAULT=("claude")
if [[ -n "${CLAUDE_CMD:-}" ]]; then
  read -r -a CLAUDE_CMD_ARR <<< "$CLAUDE_CMD"
else
  CLAUDE_CMD_ARR=("${CLAUDE_CMD_DEFAULT[@]}")
fi

MAX_ITERS_DEFAULT="${RALPH_MAX_ITERS:-1}"
SLEEP_SECS="${RALPH_LOOP_SLEEP:-0}"

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf is required for selection." >&2
  echo "Install fzf and rerun." >&2
  exit 1
fi

agent="$(
  printf "claude\ncodex\n" \
    | fzf --prompt="Agent> " --height=40% --reverse --border --select-1 --exit-0
)"

if [[ -z "$agent" ]]; then
  echo "No agent selected." >&2
  exit 1
fi

case "$agent" in
  claude|codex) ;;
  *)
    echo "Unknown agent: $agent" >&2
    exit 1
    ;;
esac

if [[ "$agent" == "claude" && -z "${CLAUDE_MODEL:-}" ]]; then
  CLAUDE_MODEL="$(
    printf "%s\n" "${CLAUDE_MODELS[@]}" \
      | fzf --prompt="Claude model> " --height=40% --reverse --border --select-1 --exit-0
  )"
  if [[ -z "$CLAUDE_MODEL" ]]; then
    echo "No Claude model selected." >&2
    exit 1
  fi
fi

shopt -s nullglob
md_files=("$ROOT_DIR"/*.md)
shopt -u nullglob

if [[ ${#md_files[@]} -eq 0 ]]; then
  echo "No .md files found in $ROOT_DIR" >&2
  exit 1
fi

selection="$(
  printf "%s\n" "${md_files[@]}" \
    | sed "s|^$ROOT_DIR/||" \
    | fzf --prompt="Prompt file> " --height=40% --reverse --border --select-1 --exit-0 --query="prompt.md"
)"

if [[ -z "$selection" ]]; then
  echo "No prompt file selected." >&2
  exit 1
fi

PROMPT_FILE="$ROOT_DIR/$selection"

while true; do
  read -r -p "Iterations [$MAX_ITERS_DEFAULT]: " input_iters
  if [[ -z "$input_iters" ]]; then
    input_iters="$MAX_ITERS_DEFAULT"
  fi
  if [[ "$input_iters" =~ ^[0-9]+$ ]] && (( input_iters >= 1 )); then
    MAX_ITERS="$input_iters"
    break
  fi
  echo "Iterations must be a whole number >= 1." >&2
done

for ((iteration=1; iteration<=MAX_ITERS; iteration++)); do
  echo "Ralph loop iteration $iteration/$MAX_ITERS ($agent)"
  case "$agent" in
    codex)
      "${CODEX_CMD_ARR[@]}" "${CODEX_EXEC_MODE_ARGS[@]}" --model "$CODEX_MODEL" - < "$PROMPT_FILE"
      ;;
    claude)
      "${CLAUDE_CMD_ARR[@]}" "${CLAUDE_EXEC_MODE_ARGS[@]}" --model "$CLAUDE_MODEL" -p "$(cat "$PROMPT_FILE")" \
        | jq -r --unbuffered '
            if .type == "system" and .subtype == "init" then
              "[system] session=\(.session_id // "?") model=\(.model // "?")"
            elif .type == "assistant" then
              ( .message.content[]? |
                if .type == "text" then
                  (.text // "" | if length > 0 then "[claude] " + . else empty end)
                elif .type == "thinking" then
                  ("[thinking] " + ((.thinking // "") | gsub("\n"; " ") | .[0:200]))
                elif .type == "tool_use" then
                  ("[tool] " + (.name // "?") + " " + ((.input // {}) | tostring | .[0:200]))
                else empty end
              )
            elif .type == "user" then
              ( .message.content[]? |
                if .type == "tool_result" then
                  ( (.content // "") |
                    if type == "array" then
                      (map(.text // "") | join(" ") | gsub("\n"; " ") | .[0:200])
                    else (tostring | gsub("\n"; " ") | .[0:200]) end
                  ) as $r
                  | "[result] " + $r
                else empty end
              )
            elif .type == "result" then
              "[done] " + (.subtype // "?") + " duration=\(.duration_ms // 0)ms cost=$\(.total_cost_usd // 0)"
            else empty end
          '
      ;;
  esac

  if [[ "$SLEEP_SECS" -gt 0 ]]; then
    sleep "$SLEEP_SECS"
  fi
done
