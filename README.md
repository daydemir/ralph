# Ralph 2.0

Autonomous plan execution engine built on [Get Shit Done (GSD)](https://github.com/glittercowboy/get-shit-done) planning.

Ralph wraps GSD's disciplined planning workflow and adds automated execution with verification. **Planning is mandatory** - Ralph enforces that you properly understand and plan work before executing.

## Installation

### Prerequisites

1. **Claude Code CLI** - [Install Claude Code](https://claude.ai/code)
2. **GSD (Get Shit Done)** - Required for planning:
   ```bash
   npx get-shit-done-cc --global
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
# 1. Initialize project (creates PROJECT.md)
ralph init

# 2. Map existing codebase (for brownfield projects)
ralph map

# 3. Create roadmap with phases
ralph roadmap

# 4. Create executable plans for Phase 1
ralph plan 1

# 5. Execute plans one at a time
ralph run

# 6. Or run autonomous loop
ralph run --loop 5
```

## Commands

### Planning Commands

| Command | Description |
|---------|-------------|
| `ralph init` | Initialize project with GSD (creates PROJECT.md) |
| `ralph roadmap` | Create phase breakdown (creates ROADMAP.md) |
| `ralph map` | Analyze existing codebase structure |
| `ralph discover [N]` | Research phase N before planning |
| `ralph discuss [N]` | Discuss phase N approach |
| `ralph plan [N]` | Create executable PLAN.md for phase N |

### Execution Commands

| Command | Description |
|---------|-------------|
| `ralph run` | Execute the next incomplete plan |
| `ralph run --loop [N]` | Autonomous loop up to N plans (default 10) |
| `ralph status` | Show current position and progress |
| `ralph list` | List all phases and plans |

### Roadmap Modification

| Command | Description |
|---------|-------------|
| `ralph add-phase "desc"` | Add phase to end of roadmap |
| `ralph insert-phase N "desc"` | Insert urgent work as phase N.1 |
| `ralph remove-phase N` | Remove phase N and renumber |

## Enforced Workflow

Ralph requires proper planning before execution. You can't skip steps:

```
ralph init      → REQUIRED first (can't run anything without PROJECT.md)
ralph roadmap   → REQUIRED (can't plan phases without ROADMAP.md)
ralph plan N    → REQUIRED (can't run without PLAN.md files)
ralph run       → Only works if valid PLAN.md exists
```

This ensures you understand the full scope before writing code.

## Workspace Structure

Ralph creates a `.planning/` directory:

```
.planning/
├── PROJECT.md          # Project vision and requirements
├── ROADMAP.md          # Phase breakdown
├── STATE.md            # Current position and progress
├── config.json         # Configuration
├── codebase/           # Codebase analysis (from ralph map)
│   ├── STACK.md
│   ├── ARCHITECTURE.md
│   └── ...
└── phases/
    ├── 01-foundation/
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

Ralph runs verification after each task. If verification fails:
- Retries once
- If still fails, stops immediately
- Reports exactly what failed

### Inactivity Timeout

Ralph monitors for 60 minutes of **inactivity** (no output), not total duration. Long builds and tests are fine as long as there's activity.

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

## Configuration

Edit `.planning/config.json`:

```json
{
  "verification": {
    "inactivity_timeout_minutes": 60,
    "retry_on_failure": true
  },
  "claude": {
    "model": "sonnet",
    "binary": "claude"
  }
}
```

## Previous Version

The v0.x PRD-based system is archived in `archive/v0-shell/`.

## License

MIT
