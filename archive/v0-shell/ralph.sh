#!/bin/bash
set -e

# Parse command-line flags
LLM_PROVIDER="claude"  # Default
MAX_ITERATIONS=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --llm)
      LLM_PROVIDER="$2"
      shift 2
      ;;
    *)
      if [[ "$1" =~ ^[0-9]+$ ]]; then
        MAX_ITERATIONS="$1"
      fi
      shift
      ;;
  esac
done

MAX_ITERATIONS=${MAX_ITERATIONS:-10}

# Validate LLM provider
if [[ "$LLM_PROVIDER" != "claude" && "$LLM_PROVIDER" != "mistral" ]]; then
  echo "Error: --llm must be 'claude' or 'mistral'"
  echo "Usage: ./ralph.sh [MAX_ITERATIONS] [--llm claude|mistral]"
  exit 1
fi

cd "$(dirname "$0")"

# Check for jq
if ! command -v jq &> /dev/null; then
  echo "Error: jq required. Install with: brew install jq"
  exit 1
fi

# Provider configuration
if [[ "$LLM_PROVIDER" == "claude" ]]; then
  CLAUDE_CMD="${CLAUDE_CMD:-claude}"

  if ! command -v "$CLAUDE_CMD" &> /dev/null; then
    echo "Error: Claude Code CLI not found."
    echo "Install from: https://claude.ai/code"
    echo "Or set CLAUDE_CMD to your claude binary path."
    exit 1
  fi

  PROMPT_FILE="PROMPT.md"

elif [[ "$LLM_PROVIDER" == "mistral" ]]; then
  if ! command -v vibe &> /dev/null; then
    echo "Error: Mistral Vibe CLI not found"
    echo "Install with: curl -LsSf https://mistral.ai/vibe/install.sh | bash"
    exit 1
  fi

  PROMPT_FILE="PROMPT-mistral.md"

  if [ -z "$MISTRAL_API_KEY" ]; then
    echo "Error: MISTRAL_API_KEY not set"
    echo "Add to ~/.zshrc: export MISTRAL_API_KEY=\"your-key-here\""
    exit 1
  fi
fi

for ((i=1; i<=MAX_ITERATIONS; i++)); do
  echo ""
  echo "=== Ralph iteration $i of $MAX_ITERATIONS (provider: $LLM_PROVIDER, started $(date +%H:%M:%S)) ==="
  echo ""

  # Show remaining PRDs before each iteration (re-reads file each time)
  REMAINING=$(jq -r '.features[] | select(.passes == false) | .id' prd.json 2>/dev/null)
  COUNT=$(echo "$REMAINING" | grep -c . || echo 0)

  if [ "$COUNT" -eq 0 ]; then
    echo "All PRDs complete!"
    echo "RALPH_COMPLETE"
    exit 0
  fi

  echo "PRDs ($COUNT remaining):"
  echo "$REMAINING" | while read -r id; do
    [ -n "$id" ] && echo "  ○ $id"
  done
  echo ""

  # Run provider-specific command
  if [[ "$LLM_PROVIDER" == "claude" ]]; then
    # Claude Code (existing)
    "$CLAUDE_CMD" --model sonnet -p "$(cat $PROMPT_FILE)" \
      --allowedTools "Read,Write,Edit,Bash,Glob,Grep,Task,TodoWrite,WebFetch,WebSearch" \
      --output-format stream-json --verbose \
      prd.json progress.txt fix_plan.md CODEBASE-MAP.md \
      2>&1 | tee /tmp/ralph-output-$i.txt | jq -r --unbuffered '
        if .type == "assistant" then
          .message.content[]? |
          if .type == "tool_use" then "TOOL"
          elif .type == "text" then
            # Check for SELECTED_PRD pattern
            if (.text | test("SELECTED_PRD:")) then
              "PRD:" + (.text | capture("SELECTED_PRD:\\s*(?<id>[a-zA-Z0-9_-]+)") | .id // "unknown")
            else
              "TEXT:" + (.text | gsub("\n"; " ") | .[0:400])
            end
          else empty end
        elif .type == "result" then
          "DONE:" + (.result | gsub("\n"; " ") | .[0:200])
        else empty end
      ' 2>/dev/null | awk '
BEGIN { tools = 0 }
/^TOOL$/ { tools++; next }
/^PRD:/ {
  prd = substr($0, 5)
  printf "\n>>> WORKING ON: %s <<<\n\n", prd
  next
}
/^TEXT:/ {
  text = substr($0, 6)
  "date +[%H:%M:%S]" | getline ts; close("date +[%H:%M:%S]")
  if (tools > 0) { printf "%s [Tools: %d] %s\n", ts, tools, text; tools = 0 }
  else { printf "%s %s\n", ts, text }
}
/^DONE:/ {
  "date +[%H:%M:%S]" | getline ts; close("date +[%H:%M:%S]")
  printf "%s [Done] %s...\n", ts, substr($0, 6)
}
END { if (tools > 0) printf "[Final tools: %d]\n", tools }
'

  elif [[ "$LLM_PROVIDER" == "mistral" ]]; then
    # Mistral Vibe CLI
    vibe --prompt "$(cat $PROMPT_FILE)" \
      2>&1 | tee /tmp/ralph-output-$i.txt | while IFS= read -r line; do
        timestamp=$(date +[%H:%M:%S])
        echo "$timestamp $line"

        if echo "$line" | grep -q "SELECTED_PRD:"; then
          prd_id=$(echo "$line" | sed -n 's/.*SELECTED_PRD:[[:space:]]*\([a-zA-Z0-9_-]*\).*/\1/p')
          echo ""
          echo ">>> WORKING ON: $prd_id <<<"
          echo ""
        fi
      done
  fi

  echo ""

  if grep -q "###RALPH_COMPLETE###" /tmp/ralph-output-$i.txt; then
    echo "=== Ralph complete after $i iterations ==="
    exit 0
  fi

  if grep -q "###ITERATION_COMPLETE###" /tmp/ralph-output-$i.txt; then
    echo "=== Iteration $i complete ==="
    continue
  fi
done

echo "=== Ralph stopped at max iterations ($MAX_ITERATIONS) ==="
