# Ralph GSD

> [!WARNING]
> **Claude Code Required** — This tool requires an active Claude Code subscription.
>
> **Cost Warning** — Running plans consumes Claude API usage. Autonomous loops can use significant quota.
>
> **Auto-Accept Mode** — Ralph runs Claude with `--dangerously-skip-permissions` enabled. It will make changes without confirmation prompts.
>
> **Recommendation** — Start with `ralph run` (single plan) to observe behavior before using `ralph run --loop`.

> Built on [Get Shit Done (GSD)](https://github.com/glittercowboy/get-shit-done) planning and inspired by the [original Ralph concept](https://ghuntley.com/ralph/) by Geoffrey Huntley.

Ralph executes your development plans automatically. You define what to build, Ralph breaks it into phases, creates detailed task plans, and executes them with verification. Each run preserves learnings in a `## Progress` section within the PLAN.md file for the next run.

## Table of Contents
- [How It Works](#how-it-works)
- [When to Use Ralph](#when-to-use-ralph)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

**Planning is mandatory** - Ralph enforces that you properly understand and plan work before executing.

## How It Works

Ralph uses a **phase-by-phase workflow**. You don't plan everything upfront - you work through one phase at a time:

```
┌───────────────────────────────────────────────────────────┐
│  ONE-TIME SETUP                                           │
│                                                           │
│  ralph init  →  ralph map  →  ralph roadmap               │
│                                                           │
└───────────────────────────────────────────────────────────┘
                          │
                          ▼
┌───────────────────────────────────────────────────────────┐
│  PER-PHASE LOOP (repeat for each phase)                   │
│                                                           │
│  ┌──────────┐     ┌──────────┐    ┌─────────┐    ┌─────┐  │
│  │ discover │  →  │ discuss  │ →  │  plan   │ →  │ run │  │
│  │(optional)│     │(optional)│    │         │    │     │  │
│  └──────────┘     └──────────┘    └─────────┘    └─────┘  │
│                                                           │
│  Phase 1 → Phase 2 → Phase 3 → ... → Done                 │
│                                                           │
└───────────────────────────────────────────────────────────┘

         At any time: ralph status (see where you are)
```

## When to Use Ralph

**Good for:**
- Greenfield projects building from scratch
- Major refactors with clear phases
- Feature development with sequential steps
- Any work that can be broken into verifiable tasks

**Not for:**
- Quick one-off changes (use Claude directly)
- Exploratory work where scope is unclear
- Work requiring constant human judgment
- Debugging sessions or investigations

## Ralph vs GSD

| Component | Role |
|-----------|------|
| **GSD** | Planning brain - interactive workflows for project setup, roadmapping, and plan creation |
| **Ralph** | Execution engine - autonomous execution with verification, looping, and progress tracking |

Ralph wraps GSD commands in a simpler CLI (`ralph plan` calls `/gsd:plan-phase`, etc.) and adds:
- Verification after each task
- Autonomous looping across plans
- Progress tracking via `ralph status`
- Inactivity timeout protection

You can use GSD directly within Claude Code for interactive planning, or use Ralph for automated execution.

## Checking Progress: `ralph status`

Run `ralph status` anytime to see your current position:

```bash
$ ralph status

Ralph v2.0.0 - My Project

📦 Project Artifacts:
  ✓ PROJECT.md          Project vision and requirements
  ✓ ROADMAP.md          10 phases defined
  ✓ Codebase Maps       7 analysis documents
  ✓ STATE.md            Tracking execution
  ✓ Plans               3/10 phases have plans

Progress: [████░░░░░░░░░░░░░░░░] 20% (4/20 plans)

📍 Current Position:
  Phase: 2 of 10
  Plan:  01-02-PLAN.md
  Status: Ready to execute

🎯 Suggested Next Actions:
  ralph run              Execute next incomplete plan
  ralph run --loop 5     Execute up to 5 plans autonomously
```

Use `ralph status -v` (verbose) to see all phases and their plans:

```bash
$ ralph status -v
...
Phases:
  ✓ Phase 1: Feature Verification (3/3)
      ✓ 01-01-PLAN.md
      ✓ 01-02-PLAN.md
      ✓ 01-03-PLAN.md
  ◐ Phase 2: Mix World Media (1/2)
      ✓ 02-01-PLAN.md
      ○ 02-02-PLAN.md
  ○ Phase 3: Story Adaptation (0/0)
  ...
```

Use `ralph list` for a compact view of all phases and plans.

## Installation

### Prerequisites

1. **Claude Code CLI** - [Install Claude Code](https://claude.ai/code)
2. **GSD (Get Shit Done)** - Required for planning:
   ```bash
   npm install -g get-shit-done-cc
   ```

### Install Ralph

**Homebrew (recommended):**
```bash
brew tap daydemir/tap
brew install ralph
```

**From source:**
```bash
go install github.com/daydemir/ralph/cmd/ralph@latest
```

## Quick Start

```bash
# One-time setup
ralph init              # Create PROJECT.md
ralph map               # Analyze existing codebase (brownfield projects)
ralph roadmap           # Create ROADMAP.md with phases

# Per-phase cycle
ralph discover 1        # Research external APIs/docs (optional)
ralph discuss 1         # Align on scope and approach (optional)
ralph plan 1            # Create executable PLAN.md files
ralph run               # Execute plans (or: ralph run --loop 5)
```

## Workflow Example

Here's a complete cycle for one phase:

```bash
# Phase 1: Authentication
ralph discover 1        # → Creates .planning/phases/01-auth/RESEARCH.md
                        #   Researches OAuth providers, JWT libraries, etc.

ralph discuss 1         # → Creates .planning/phases/01-auth/CONTEXT.md
                        #   Alignment conversation about scope, edge cases

ralph plan 1            # → Creates .planning/phases/01-auth/01-01-PLAN.md, etc.
                        #   Breaks phase into executable task files

ralph run --loop        # Executes all plans for phase 1
                        # STATE.md updates automatically

# Phase 1 complete → now repeat for Phase 2
ralph discover 2
ralph discuss 2
ralph plan 2
ralph run --loop
```

## Pre-Planning: Discover vs Discuss

Before planning a phase, you can optionally run **discover** and/or **discuss** to build context:

| Command | Purpose | Output | When to Use |
|---------|---------|--------|-------------|
| `ralph discover N` | Research external docs, APIs, ecosystem options | `RESEARCH.md` | Unfamiliar domain, new libraries, API integrations |
| `ralph discuss N` | Alignment conversation about scope/approach | `CONTEXT.md` | Complex decisions, unclear requirements, multiple approaches |

**Guidelines:**
- **Familiar domain?** Skip discover, maybe run discuss for alignment
- **New technology?** Run discover to research options first
- **Complex phase?** Run both - discover first, then discuss with that context
- **Simple phase?** Skip both and go straight to `ralph plan`

Both commands are optional but recommended for non-trivial phases. The context they generate helps `ralph plan` create better plans.

## Interactive Session Workflow

Commands `ralph discover`, `ralph discuss`, and `ralph plan` open **interactive Claude sessions**. Here's how to use them:

1. **Run the command** - A Claude conversation starts
2. **Discuss with Claude** - Work through the topic until Claude indicates completion
3. **Look for the completion signal** - Claude will say something like:
   - "I've updated CONTEXT.md with our discussion"
   - "I've created the PLAN.md files"
   - "RESEARCH.md has been saved"
4. **Exit the session** - Type `/exit` or press `Ctrl+C`
5. **Check progress** - Run `ralph status` to see your updated state

**Tip:** These sessions are conversational. Ask follow-up questions, request changes, or explore alternatives before Claude finalizes the output. Once Claude confirms the file has been written, your work is saved and you can safely exit.

## Commands

### Setup Commands (One-Time)

| Command | Description |
|---------|-------------|
| `ralph init` | Initialize project with GSD (creates PROJECT.md) |
| `ralph roadmap` | Create phase breakdown (creates ROADMAP.md) |
| `ralph map` | Analyze existing codebase structure |

### Pre-Planning Commands (Optional)

| Command | Description |
|---------|-------------|
| `ralph discover [N]` | Research phase N - external docs, APIs, options → RESEARCH.md |
| `ralph discuss [N]` | Discuss phase N - scope and approach alignment → CONTEXT.md |

### Planning Commands

| Command | Description |
|---------|-------------|
| `ralph plan [N]` | Create executable PLAN.md files for phase N |

### Execution Commands

| Command | Description |
|---------|-------------|
| `ralph run` | Execute the next incomplete plan |
| `ralph run --loop [N]` | Autonomous loop up to N plans (default 10) |
| `ralph run --model MODEL` | Use specific model (sonnet, opus, haiku) |
| `ralph status` | Dashboard: current phase, progress, suggested actions |

Model options:
- **sonnet** (default): Best balance of speed and capability
- **opus**: More capable but slower, for complex phases
- **haiku**: Fastest, for simple repetitive tasks
| `ralph status -v` | Verbose: show all phases and plans with completion status |
| `ralph list` | Compact list of all phases and plans |

### Roadmap Modification

| Command | Description |
|---------|-------------|
| `ralph add-phase "desc"` | Add phase to end of roadmap |
| `ralph insert-phase N "desc"` | Insert urgent work as phase N.1 |
| `ralph remove-phase N` | Remove phase N and renumber |

**Planning is mandatory** - Ralph enforces that you properly understand and plan work before executing.

## Workspace Structure

Ralph creates two directories:

```
.ralph/
└── config.yaml         # Ralph configuration (optional)

.planning/              # Created by GSD
├── PROJECT.md          # Project vision and requirements
├── ROADMAP.md          # Phase breakdown
├── STATE.md            # Current position and progress
├── codebase/           # Codebase analysis (from ralph map)
│   ├── STACK.md
│   ├── ARCHITECTURE.md
│   └── ...
└── phases/
    ├── 01-foundation/
    │   ├── RESEARCH.md       # From ralph discover (optional)
    │   ├── CONTEXT.md        # From ralph discuss (optional)
    │   ├── 01-01-PLAN.md
    │   ├── 01-01-SUMMARY.md
    │   └── 01-02-PLAN.md
    └── 02-authentication/
        └── ...
```

## Execution with Verification

Each PLAN.md contains tasks with verification commands:

```xml
<task type="auto">
  <name>Create user model</name>
  <action>Create User struct with fields</action>
  <verify>
    <command>go build ./...</command>
    <expect>exit 0</expect>
  </verify>
</task>
```

Ralph runs verification after each task. If verification fails, execution stops immediately and reports what failed.

### Inactivity Timeout

Ralph monitors for 60 minutes of **inactivity** (no output), not total duration. Long builds and tests are fine as long as there's activity.

### Context Management

Claude's context degrades after ~100K tokens. Ralph uses a hybrid approach:

1. **Progress tracking**: Claude updates a `## Progress` section in each PLAN.md after completing tasks
2. **Self-monitoring**: Claude is instructed to bail out gracefully at ~100K tokens
3. **Safety net**: Ralph terminates at 120K tokens if Claude hasn't bailed out

When context runs low, Ralph preserves learnings in the PLAN.md file so the next run can continue where it left off.

## Autonomous Loop

`ralph run --loop` executes multiple plans with:
- Fresh Claude context per plan (200k tokens)
- Automatic verification between plans
- Immediate stop on failure
- Progress tracking in STATE.md

```bash
$ ralph run --loop 5
=== Ralph Autonomous Loop ===

Iteration 1/5: 01-01-PLAN.md
[12:34:56] Executing: Foundation setup
[12:35:12] ✓ Task 1/3 complete
[12:36:01] ✓ Task 2/3 complete
[12:37:15] ✓ Task 3/3 complete
✓ Complete (2m 19s)

Iteration 2/5: 01-02-PLAN.md
...
```

## Post-Plan Analysis

After each plan completes, Ralph runs an **analysis agent** that reviews discoveries and can adjust subsequent plans.

### What It Does

The analysis agent:
- Reads `## Discoveries` section from the completed PLAN.md
- Reviews all remaining plans in the current and future phases
- Updates plans based on findings (adds context, blockers, notes)
- Runs even on failures to help diagnose issues

### Discovery Types

During execution, Claude records discoveries in XML format:

| Type | Description |
|------|-------------|
| `bug` | Existing bug found in codebase |
| `stub` | Tests or code that are placeholders |
| `api-issue` | External API behaving unexpectedly |
| `insight` | Useful pattern or approach discovered |
| `blocker` | Something preventing progress |
| `technical-debt` | Code quality issue found |
| `tooling-friction` | Build/test quirks learned through trial-and-error |
| `assumption` | Decision made without full information |
| `scope-creep` | Work discovered that wasn't in the plan |
| `dependency` | Unexpected dependency between tasks |
| `questionable` | Suspicious code or pattern worth reviewing |

### When Analysis Runs

The analysis agent runs:
- After successful plan completion
- After soft failures (context exhaustion, bailout with progress)
- After hard failures (task/verification failure) - to diagnose issues

### Skipping Analysis

Use `--skip-analysis` to disable post-plan analysis:

```bash
ralph run --skip-analysis
ralph run --loop 5 --skip-analysis
```

## Configuration

Edit `.ralph/config.yaml`:

```yaml
llm:
  backend: claude      # LLM backend to use
  model: sonnet        # Model: sonnet, opus, or haiku

claude:
  binary: claude       # Path to Claude CLI binary
  allowed_tools:       # Tools Claude can use during execution
    - Read
    - Write
    - Edit
    - Bash
    - Glob
    - Grep
    - Task
    - TodoWrite
    - WebFetch
    - WebSearch

build:
  default_loop_iterations: 10    # Default max iterations for --loop
  signals:
    iteration_complete: "###ITERATION_COMPLETE###"
    ralph_complete: "###RALPH_COMPLETE###"
```

Ralph uses sensible defaults if no config file exists.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "GSD not installed" error | Run `npm install -g get-shit-done-cc` to install GSD |
| "No ROADMAP.md found" | Run `ralph init` then `ralph roadmap` first |
| Plan execution fails | Run `ralph status -v` to see current state, check the PLAN.md file for issues, fix manually then retry |
| "Context exceeded" errors | Plan may be too large - break the phase into smaller sub-phases |
| Claude hangs or times out | Check network connection; Ralph has 60-minute inactivity timeout (not total time) |
| Wrong phase executing | Check STATE.md in `.planning/` - manually edit if needed to reset position |
| Ralph was interrupted mid-plan | Run `ralph status` to see state, then `ralph run` to resume |

## Previous Version

The v0.x PRD-based system is archived in `archive/v0-shell/`.

## Development

### Building from Source

```bash
# Clone and build
git clone https://github.com/daydemir/ralph.git
cd ralph
go build -o ralph ./cmd/ralph

# Install to $GOPATH/bin
go install ./cmd/ralph
```

### Releasing a New Version

Ralph uses [GoReleaser](https://goreleaser.com/) to build binaries and update the Homebrew formula.

```bash
# 1. Commit your changes
git add .
git commit -m "feat: your feature description"

# 2. Tag the new version
git tag -a v0.X.0 -m "Release description"

# 3. Push with tags
git push origin main --tags

# 4. Run GoReleaser (requires GitHub token)
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

**Note:** The `gh auth token` command uses the GitHub CLI to get your token. You need `gh auth login` first with `repo` scope.

GoReleaser will:
- Build binaries for darwin/linux (amd64/arm64)
- Create a GitHub release with changelog
- Update the Homebrew formula in `daydemir/homebrew-tap`

### Local Development Install

```bash
# Install with version (for testing before release)
go install -ldflags "-X github.com/daydemir/ralph/internal/cli.Version=dev" ./cmd/ralph
```

## License

MIT
