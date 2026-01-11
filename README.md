# Ralph

Autonomous PRD execution loop for [Claude Code](https://claude.ai/code).

> [!CAUTION]
> **Ralph runs with auto-accept enabled**
>
> Ralph scripts run Claude Code with **all tools enabled** and **changes auto-accepted** (no confirmation prompts). This means:
> - File edits happen immediately without review
> - Commands execute without asking permission
> - Git commits are made automatically
>
> **Start safe:**
> 1. Use `ralph-once.sh` first to watch Claude work interactively
> 2. Always run in a git-tracked directory so you can revert changes
> 3. Consider the [safety measures](#safety-measures) below before running autonomously

## What is Ralph?

Ralph is a system for autonomous code development using Claude Code. The workflow:

1. **You** talk to Claude (as a Product Manager) to define PRDs
2. PRDs get added to `prd.json`
3. Ralph runs Claude Code, implementing PRDs one by one
4. You can add new PRDs while Ralph works

Ralph picks a PRD, writes tests, implements the feature, verifies it works, marks it complete, and moves to the next one.

## Prerequisites

- [Claude Code CLI](https://claude.ai/code) installed and authenticated
- `jq` for JSON processing: `brew install jq` (only needed for `ralph.sh` loop)

Verify Claude Code is working:
```bash
claude --version
```

## Setup

1. **Use this template** to create your own repo (click "Use this template" on GitHub)

2. **Clone next to your project repos** (not inside them):
   ```
   ~/projects/              # Your workspace folder
   ├── ralph/               # This repo - sits NEXT TO your code repos
   ├── my-app/              # Your application repo
   └── my-backend/          # Your backend repo (if any)
   ```

   > **Important:** Ralph works by looking at sibling directories. It should be at the same level as your code repos, not inside them.

3. **Fill in `CODEBASE-MAP.md`** with your repo structure and tech stack

4. **Customize `PROMPT.md`** for your stack (optional - defaults work for most projects)

5. **Add your first PRD** to `prd.json`

6. **Run Ralph:**
   ```bash
   ./ralph-once.sh   # Interactive session (recommended to start)
   ./ralph.sh        # Autonomous loop until all PRDs complete
   ./ralph.sh 5      # Autonomous loop with max 5 iterations
   ```

## Recommended Workflow

### Step 1: Plan with Claude (PM Mode)

Before running Ralph, use Claude Code as your product manager to plan PRDs:

1. **Start Claude Code** in your workspace (the parent folder containing ralph and your repos):
   ```bash
   cd ~/projects    # The folder containing ralph/, my-app/, etc.
   claude
   ```
   This lets Claude explore all your repos when planning PRDs.

2. **Describe what you want to build:**
   ```
   You: I want to add user authentication to my app.
        Can you research the codebase and plan out what PRDs we need?
   ```

3. **Claude enters plan mode**, explores your codebase, and proposes PRDs

4. **Review and refine** the plan before Claude adds PRDs to prd.json

This approach lets Claude do the research and planning upfront, so PRDs are well-informed.

### Step 2: Run Ralph

Once PRDs are in prd.json, choose your mode:

**Interactive (recommended to start):**
```bash
./ralph-once.sh
```
Opens a Claude Code session where you can watch and interact. Claude works on one PRD, and you can ask questions, give feedback, or redirect if needed.

**Autonomous loop:**
```bash
./ralph.sh        # Loop until all PRDs complete (max 10 iterations)
./ralph.sh 5      # Loop with max 5 iterations
```
Runs Claude autonomously in a loop. You see summarized output but can't interact. Good for overnight runs or when you trust the PRDs are well-defined.

## Quick Start: Your First Feature

After setup, try this simple first PRD to see Ralph in action:

1. **Edit `CODEBASE-MAP.md`** - fill in your repo path and tech stack:
   ```markdown
   ## Repositories
   - `../my-app/` - Main application (Node.js, npm)

   ## Build & Test
   - Build: `cd ../my-app && npm run build`
   - Test: `cd ../my-app && npm test`
   ```

2. **Edit `prd.json`** - the template includes a starter PRD:
   ```json
   {
     "features": [
       {
         "id": "add-readme",
         "description": "Create a comprehensive README for the project",
         "steps": [
           "Analyze project structure and dependencies",
           "Document setup and installation instructions",
           "Add usage examples",
           "Include contribution guidelines"
         ],
         "passes": false
       }
     ]
   }
   ```

3. **Run Ralph interactively:**
   ```bash
   ./ralph-once.sh
   ```
   This opens an interactive Claude session so you can watch and intervene if needed.

4. **Watch Claude work** - you'll see it analyze your codebase, create the README, and mark the PRD complete

## What to Expect

### Interactive mode (`ralph-once.sh`)

You're in a normal Claude Code session. You can:
- Watch Claude work through the PRD
- Ask questions or give feedback
- Redirect if something goes wrong
- Exit anytime with Ctrl+C

### Autonomous mode (`ralph.sh`)

You'll see summarized output like this:

```
=== Ralph iteration 1 of 10 (started 14:32:15) ===

PRDs (1 remaining):
  ○ add-readme

>>> WORKING ON: add-readme <<<

[14:32:18] I'll analyze the project structure first...
[14:32:25] [Tools: 3] The project uses Node.js with Express...
[14:32:40] [Tools: 5] Creating the README with setup instructions...
[14:33:02] [Done] Completed add-readme, marking as passes=true...

=== Ralph complete after 1 iterations ===
```

- `>>> WORKING ON: <id> <<<` shows which PRD Ralph selected
- `[HH:MM:SS]` timestamps track progress
- `[Tools: N]` shows tool calls between text outputs
- `[Done]` indicates completion

## Safety Measures

For autonomous runs, consider these protections:

### Claude Code Safety Net Plugin

Catches destructive commands (`rm -rf`, `git reset --hard`, etc.) before they execute:
```bash
/plugin marketplace add kenryu42/cc-marketplace
/plugin install safety-net@cc-marketplace
```
See [claude-code-safety-net](https://github.com/kenryu42/claude-code-safety-net) for details.

### Built-in Sandboxing

Claude Code supports OS-level sandboxing (filesystem + network isolation). See [Anthropic's sandboxing guide](https://www.anthropic.com/engineering/claude-code-sandboxing).

### Other Options

- **Docker container**: Run Ralph in an isolated container
- **VM/separate machine**: For maximum isolation from your main environment
- **Git branch**: Work on a feature branch so main stays clean

## Adding PRDs

Edit `prd.json` to add features:

```json
{
  "features": [
    {
      "id": "user-authentication",
      "description": "Add login/logout with JWT tokens",
      "steps": [
        "Create auth middleware",
        "Add login endpoint",
        "Add logout endpoint",
        "Write tests for auth flow",
        "Update API documentation"
      ],
      "passes": false
    }
  ]
}
```

Ralph will:
- Pick a PRD where `passes: false`
- Follow the steps
- Set `passes: true` when complete
- Move to the next PRD

## File Structure

```
ralph/
├── ralph-once.sh      # Interactive session (human-in-loop)
├── ralph.sh           # Autonomous loop script
├── PROMPT.md          # Instructions for Ralph (customize for your stack)
├── prd.json           # Your PRDs (add features here)
├── progress.txt       # Ralph's memory of completed work
├── fix_plan.md        # Known bugs and issues to address
├── CODEBASE-MAP.md    # Your repo structure and tech stack
└── specs/             # Detailed feature specifications (optional)
```

## Customization

### PROMPT.md

Customize the verification commands for your stack:

```markdown
## 4. Run Verification
- Build: `npm run build`
- Test: `npm test`
- Lint: `npm run lint`
```

### Code Standards

The default PROMPT.md includes code standards (type safety, functional programming, minimal code). Modify these to match your team's practices.

## Sources & Inspiration

- [Original Ralph concept by Geoffrey Huntley](https://ghuntley.com/ralph/)
- [Ralph demo video](https://youtu.be/_IK18goX4X8?si=LSf_Mgjr9ym8pcY8)
- [Anthropic: Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)

## License

MIT
