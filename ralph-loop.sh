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

MAX_ITERS_DEFAULT="${RALPH_MAX_ITERS:-1}"
SLEEP_SECS="${RALPH_LOOP_SLEEP:-0}"

shopt -s nullglob
md_files=("$ROOT_DIR"/*.md)
shopt -u nullglob

if [[ ${#md_files[@]} -eq 0 ]]; then
  echo "No .md files found in $ROOT_DIR" >&2
  exit 1
fi

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf is required for prompt selection." >&2
  echo "Install fzf or set PROMPT_FILE directly and rerun." >&2
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
  echo "Ralph loop iteration $iteration/$MAX_ITERS"
  "${CODEX_CMD_ARR[@]}" "${CODEX_EXEC_MODE_ARGS[@]}" --model "$CODEX_MODEL" - < "$PROMPT_FILE"

  if [[ "$SLEEP_SECS" -gt 0 ]]; then
    sleep "$SLEEP_SECS"
  fi
done
