#!/bin/bash
set -e

MAX_ITERATIONS=${1:-10}

cd "$(dirname "$0")"

# Claude Code CLI - override with CLAUDE_CMD env var if needed
CLAUDE_CMD="${CLAUDE_CMD:-claude}"

# Check for jq
if ! command -v jq &> /dev/null; then
  echo "Error: jq required. Install with: brew install jq"
  exit 1
fi

# Check for claude
if ! command -v "$CLAUDE_CMD" &> /dev/null; then
  echo "Error: Claude Code CLI not found."
  echo "Install from: https://claude.ai/code"
  echo "Or set CLAUDE_CMD to your claude binary path."
  exit 1
fi

for ((i=1; i<=MAX_ITERATIONS; i++)); do
  echo ""
  echo "=== Ralph iteration $i of $MAX_ITERATIONS (started $(date +%H:%M:%S)) ==="
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

  # Run Claude and process output
  "$CLAUDE_CMD" -p "$(cat PROMPT.md)" \
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

  echo ""

  if grep -q "RALPH_COMPLETE" /tmp/ralph-output-$i.txt; then
    echo "=== Ralph complete after $i iterations ==="
    exit 0
  fi
done

echo "=== Ralph stopped at max iterations ($MAX_ITERATIONS) ==="
