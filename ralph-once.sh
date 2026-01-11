#!/bin/bash
set -e

cd "$(dirname "$0")"

CLAUDE_CMD="${CLAUDE_CMD:-claude}"

# Check for claude
if ! command -v "$CLAUDE_CMD" &> /dev/null; then
  echo "Error: Claude Code CLI not found."
  echo "Install from: https://claude.ai/code"
  echo "Or set CLAUDE_CMD to your claude binary path."
  exit 1
fi

echo "=== Ralph interactive session (human-in-loop) ==="
echo ""
"$CLAUDE_CMD" --model sonnet \
  "$(cat PROMPT.md)" \
  --allowedTools "Read,Write,Edit,Bash,Glob,Grep,Task,TodoWrite,WebFetch,WebSearch" \
  prd.json progress.txt fix_plan.md CODEBASE-MAP.md
