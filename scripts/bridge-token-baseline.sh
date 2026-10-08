#!/usr/bin/env bash
# bridge-token-baseline.sh — compare the tokens chit-claude's runs use with
# plain `claude -p` runs of the same conversation.
#
#   scripts/bridge-token-baseline.sh [workdir] [model]
#
# Runs a three-turn conversation twice in workdir (default: an empty temp
# directory), resuming the session each turn:
#
#   direct  claude -p "<message>" --output-format json [--resume <id>]
#           as a person would run it, with their own MCP servers and settings;
#   bridge  the flags internal/bridge/claude.go's args() passes by default,
#           with the prompt on stdin tagged with its author, as chit-claude
#           sends it.
#
# It prints each turn's usage and both totals. It spends real tokens: six
# small runs, on haiku unless a model is given. Keep the bridge flags below in
# step with args().
set -euo pipefail

WORKDIR="${1:-$(mktemp -d)}"
MODEL="${2:-haiku}"
MESSAGES=(
  "In one sentence, what is a write-ahead log?"
  "Give one reason it helps crash recovery."
  "Summarise our exchange in five words."
)

command -v claude >/dev/null || { echo "claude CLI not found" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq not found" >&2; exit 1; }

# run MODE SESSION MESSAGE: one turn, printing the result JSON.
run() {
  local mode=$1 session=$2 message=$3
  local resume=()
  [[ -n "$session" ]] && resume=(--resume "$session")
  if [[ "$mode" == direct ]]; then
    (cd "$WORKDIR" && claude -p "$message" --output-format json --model "$MODEL" "${resume[@]}")
  else
    (cd "$WORKDIR" && printf '[@alice] %s' "$message" | claude -p --output-format json \
      --permission-mode dontAsk --strict-mcp-config --setting-sources project \
      --model "$MODEL" --allowedTools Read,Grep,Glob "${resume[@]}")
  fi
}

printf '%-7s %4s %8s %10s %10s %7s %9s %8s\n' mode turn input "cache rd" "cache wr" output context cost
declare -A TOTAL_IN TOTAL_OUT TOTAL_COST
for mode in direct bridge; do
  session="" total_in=0 total_out=0 total_cost=0
  for i in "${!MESSAGES[@]}"; do
    json=$(run "$mode" "$session" "${MESSAGES[$i]}")
    if [[ "$(jq -r .is_error <<<"$json")" == true ]]; then
      echo "$mode turn $((i + 1)) failed: $(jq -r .result <<<"$json")" >&2
      exit 1
    fi
    session=$(jq -r .session_id <<<"$json")
    read -r input rd wr out ctx cost < <(jq -r '
      .usage as $u | ($u.iterations // [] | last // $u) as $l |
      [$u.input_tokens, $u.cache_read_input_tokens, $u.cache_creation_input_tokens,
       $u.output_tokens,
       ($l.input_tokens + $l.cache_read_input_tokens + $l.cache_creation_input_tokens),
       .total_cost_usd] | @tsv' <<<"$json")
    printf '%-7s %4d %8d %10d %10d %7d %9d %8.4f\n' "$mode" $((i + 1)) "$input" "$rd" "$wr" "$out" "$ctx" "$cost"
    total_in=$((total_in + input + rd + wr))
    total_out=$((total_out + out))
    total_cost=$(jq -n "$total_cost + $cost")
  done
  TOTAL_IN[$mode]=$total_in TOTAL_OUT[$mode]=$total_out TOTAL_COST[$mode]=$total_cost
done

echo
for mode in direct bridge; do
  printf '%-7s input (fresh + cache read + cache write) %8d · output %6d · $%.4f\n' \
    "$mode" "${TOTAL_IN[$mode]}" "${TOTAL_OUT[$mode]}" "${TOTAL_COST[$mode]}"
done
printf 'bridge / direct input: %.2f\n' "$(jq -n "${TOTAL_IN[bridge]} / ${TOTAL_IN[direct]}")"
