# Ralph 2.0: GSD + Autonomous Execution

> **Status:** Approved plan, ready for implementation
> **Date:** 2026-01-12

## Resources & References

### GSD (Get Shit Done) - The Planning Engine
- **Repository:** https://github.com/glittercowboy/get-shit-done
- **Install:** `npx get-shit-done-cc`
- **Key Files to Study:**
  - `get-shit-done/workflows/execute-phase.md` - Execution workflow
  - `get-shit-done/workflows/plan-phase.md` - Planning workflow
  - `get-shit-done/workflows/research-phase.md` - Discovery workflow
  - `get-shit-done/templates/state.md` - STATE.md structure
  - `get-shit-done/templates/summary.md` - SUMMARY.md structure
  - `get-shit-done/references/research-pitfalls.md` - Research verification
  - `get-shit-done/references/scope-estimation.md` - Task sizing (2-3 per plan)

### Claude Code
- **Documentation:** https://docs.anthropic.com/en/docs/claude-code
- **CLI Reference:** https://docs.anthropic.com/en/docs/claude-code/cli-usage
- **SDK (for spawning):** https://docs.anthropic.com/en/docs/claude-code/sdk

### Current Ralph Implementation
- **Repository:** https://github.com/daydemir/ralph (this repo)
- **Current structure:** `internal/cli/`, `internal/llm/`, `internal/prompts/`

---

## Core Architecture

**GSD** = Planning brain (use as-is, don't fork)
**Ralph** = Execution engine (autonomous loop with verification)

```
┌─────────────────────────────────────────────────────────┐
│  GSD (Planning)                                         │
│  ───────────────                                        │
│  • Structured questioning                               │
│  • Discovery with confidence levels                     │
│  • Source hierarchy (Context7 → docs → web)             │
│  • PROJECT.md, ROADMAP.md, STATE.md                     │
│  • PLAN.md generation with XML tasks                    │
│                                                         │
│  Already exists. Use it or fork it.                     │
└─────────────────────────────────────────────────────────┘
                            │
                            │ Produces .planning/ files
                            ▼
┌─────────────────────────────────────────────────────────┐
│  Ralph (Execution)                                      │
│  ─────────────────                                      │
│  • Reads GSD's .planning/ structure                     │
│  • Spawns Claude Code with fresh context per plan       │
│  • Automated verification after each task               │
│  • Loops until done or failure                          │
│  • Updates STATE.md, creates SUMMARY.md                 │
│                                                         │
│  THIS IS WHAT WE BUILD.                                 │
└─────────────────────────────────────────────────────────┘
```

## CLI Design

### Simple Commands

```bash
# Planning (wraps GSD)
ralph init              # → /gsd:new-project
ralph roadmap           # → /gsd:create-roadmap
ralph discover [phase]  # → /gsd:research-phase
ralph plan [phase]      # → /gsd:plan-phase

# Execution (Ralph's core)
ralph run               # Execute next incomplete plan
ralph run --loop        # Autonomous loop: all plans, fresh context each
ralph run --loop 5      # Max 5 iterations

# Status
ralph status            # Show STATE.md position + next action
```

### How `ralph run --loop` Works

```
1. Read .planning/STATE.md → find current position
2. Read .planning/ROADMAP.md → find next incomplete plan
3. If no incomplete plans → DONE
4. Spawn Claude Code instance:
   - Fresh context (no conversation history)
   - Pass: plan file path + verification prompt
   - Allowed tools: Read, Write, Edit, Bash, Glob, Grep, Task, TodoWrite
5. Claude executes plan:
   - Runs each task
   - Runs verification after each task
   - Commits per task
   - Creates SUMMARY.md
   - Signals completion: ###PLAN_COMPLETE###
6. Ralph validates:
   - SUMMARY.md exists?
   - Verification commands pass?
   - No ###PLAN_FAILED### signal?
7. Update STATE.md
8. Loop back to step 1 (fresh context)
```

## Verification: The Critical Part

GSD has checkpoints but they're human-interactive. Ralph needs **automated verification**.

### Task-Level Verification

Each task in PLAN.md has a `<verify>` element:
```xml
<task type="auto">
  <name>Create user model</name>
  <action>Create User struct with email, password_hash fields</action>
  <verify>
    <command>go build ./...</command>
    <expect>exit 0</expect>
  </verify>
  <done>User model compiles without errors</done>
</task>
```

Ralph runs the verify command and checks the result before continuing.

### Plan-Level Verification

Each PLAN.md has a `<verification>` section:
```xml
<verification>
  <check name="build">
    <command>go build ./...</command>
    <expect>exit 0</expect>
  </check>
  <check name="tests">
    <command>go test ./...</command>
    <expect>exit 0</expect>
  </check>
  <check name="lint">
    <command>golangci-lint run</command>
    <expect>exit 0</expect>
  </check>
</verification>
```

Ralph runs ALL verification checks before marking plan complete.

### Failure Handling

```
If task verification fails:
  → Retry task once
  → If still fails: mark plan as BLOCKED, stop loop, report

If plan verification fails:
  → Don't create SUMMARY.md
  → Signal ###PLAN_FAILED###
  → Stop loop, report what failed

If Claude crashes/hangs:
  → Timeout after N minutes
  → Mark as BLOCKED
  → Report and stop
```

## GSD Integration: Depend on GSD (Chosen)

```bash
# Prerequisites
# User must have GSD installed: https://github.com/glittercowboy/get-shit-done
# Ralph planning commands shell out to Claude with GSD slash commands

ralph init  →  claude "/gsd:new-project"
ralph roadmap  →  claude "/gsd:create-roadmap"
ralph discover 1  →  claude "/gsd:research-phase 1"
ralph plan 1  →  claude "/gsd:plan-phase 1"
```

**Why this approach:**
- No fork maintenance
- Get GSD updates automatically
- Ralph stays focused on execution
- Single responsibility: GSD plans, Ralph executes

## Checkpoint Handling: Skip (Chosen)

Plans may have `type="checkpoint:*"` tasks (human verification/decision points).

**In autonomous mode (`ralph run --loop`):**
- Skip all checkpoint tasks
- Execute only `type="auto"` tasks
- Log skipped checkpoints in SUMMARY.md
- Continue to next task

**Why:** Autonomous execution shouldn't block on human input. If a plan needs human verification, run it manually with `ralph run` (single plan, interactive).

## Project Structure

```
ralph/
├── cmd/ralph/main.go
├── internal/
│   ├── executor/         # The autonomous loop
│   │   ├── loop.go       # Main execution loop
│   │   ├── verify.go     # Verification runner
│   │   └── claude.go     # Claude Code spawner
│   ├── planner/          # GSD integration
│   │   └── gsd.go        # Calls GSD slash commands
│   └── state/            # STATE.md parsing
│       └── state.go
└── prompts/
    └── execute.md        # Execution prompt with verification focus
```

## Implementation Phases

### Phase 1: Core Executor
1. Create `executor/loop.go` - main loop logic
2. Create `executor/claude.go` - spawn Claude Code instances
3. Create `executor/verify.go` - run verification commands
4. Create `state/state.go` - parse/update STATE.md

### Phase 2: GSD Integration
1. Create `planner/gsd.go` - wrap GSD slash commands
2. Wire up `ralph init/roadmap/discover/plan` → GSD

### Phase 3: CLI
1. Create cobra CLI with commands
2. `ralph run` - single plan execution
3. `ralph run --loop` - autonomous loop
4. `ralph status` - show position

### Phase 4: Verification Enhancements
1. Parse `<verify>` from PLAN.md tasks
2. Parse `<verification>` from PLAN.md
3. Run checks, handle failures
4. Timeout handling

## Files to Create

| File | Purpose |
|------|---------|
| `cmd/ralph/main.go` | CLI entrypoint |
| `internal/executor/loop.go` | Autonomous execution loop |
| `internal/executor/claude.go` | Spawn Claude Code instances |
| `internal/executor/verify.go` | Run verification commands |
| `internal/planner/gsd.go` | GSD slash command wrapper |
| `internal/state/state.go` | STATE.md parser |
| `prompts/execute.md` | Execution prompt for Claude |

## The Execute Prompt (Critical)

```markdown
You are executing a plan autonomously. No human is watching.

## Your Mission
Execute all tasks in the plan. Verify each one. Commit atomically.

## Plan Location
{plan_path}

## Verification Protocol
After EVERY task:
1. Run the <verify> command from the task
2. If it fails → retry the task ONCE
3. If still fails → signal ###TASK_FAILED:{task_name}### and STOP

After ALL tasks:
1. Run every check in <verification> section
2. ALL must pass before creating SUMMARY.md
3. If any fail → signal ###PLAN_FAILED:{check_name}### and STOP

## Commit Protocol
After each task passes verification:
```bash
git add <files>
git commit -m "{type}({phase}-{plan}): {task_name}"
```

## Completion Signal
When ALL tasks done AND ALL verification passes:
1. Create SUMMARY.md in phase directory
2. Signal: ###PLAN_COMPLETE###

## Failure Signals
- ###TASK_FAILED:{name}### - Task couldn't complete
- ###PLAN_FAILED:{check}### - Plan verification failed
- ###BLOCKED:{reason}### - Can't continue, need human

## Rules
- NO placeholders or stubs
- NO skipping verification
- NO continuing after failure
- If uncertain → ###BLOCKED:uncertain###
```

## Acceptance Criteria

After implementation:
1. `ralph init` opens Claude with GSD init workflow
2. `ralph status` shows current position from STATE.md
3. `ralph run` executes one plan with verification
4. `ralph run --loop` executes multiple plans, fresh context each
5. Failed verification stops the loop and reports clearly

## Decisions Made

| Question | Decision |
|----------|----------|
| GSD integration | Depend on GSD (user installs separately) |
| Checkpoint handling | Skip in autonomous mode |
| Timeout | 30 min per plan (configurable) |
| Retry logic | Retry failed task once, then stop |

## Prerequisites

User must have installed:
1. **Claude Code** - https://docs.anthropic.com/en/docs/claude-code
2. **GSD** - https://github.com/glittercowboy/get-shit-done

Ralph will check for these on startup and error with install instructions if missing.

---

## Key Learnings from GSD Analysis

These insights came from deep analysis of GSD's codebase. The implementing agent should internalize these:

### 1. Context Degradation is Real
GSD's `scope-estimation.md` documents that Claude quality degrades at ~50% context usage. This is why:
- Plans are limited to 2-3 tasks
- Each execution gets fresh context
- The loop pattern with context reset is essential, not optional

### 2. Verification Must Be Automated
GSD has interactive checkpoints (`checkpoint:human-verify`, `checkpoint:decision`). These work for human-supervised execution but break autonomous loops. Ralph's innovation is:
- Replace interactive verification with command-based verification
- `<verify><command>go test ./...</command><expect>exit 0</expect></verify>`
- Machine-checkable, no human needed

### 3. State Persistence Matters
GSD's STATE.md tracks:
- Current position (which phase, which plan)
- Accumulated decisions (constraints on future work)
- Deferred issues (things to address later)
- Session continuity (where to resume)

Ralph must update STATE.md after each plan execution so the loop knows where to continue.

### 4. Signals Must Be Unambiguous
The `###PLAN_COMPLETE###` signal pattern is critical. Claude's output is messy - you need a clear marker to parse. Consider:
- Put signals on their own line
- Use unique patterns that won't appear in normal output
- Parse from the END of output (Claude often adds commentary after signals)

### 5. Git Integration Per Task, Not Per Plan
GSD commits after EACH task, not after the whole plan. Benefits:
- `git bisect` finds exact failing task
- Each task independently revertable
- Clear history for future Claude sessions

---

## Anti-Patterns to Avoid

### Don't: Run Everything in One Context
The old Ralph v1 approach of loading everything upfront leads to context exhaustion. Fresh context per plan is mandatory.

### Don't: Trust Claude's "I'm done" Without Verification
Claude will say "Task complete!" without actually completing. Always run verification commands.

### Don't: Continue After Failure
If a task fails, STOP. Don't try to "work around it" or "come back to it later." The loop should halt and report.

### Don't: Parse GSD Files Manually
GSD's file formats may change. Use simple patterns:
- STATE.md: Look for `Phase: X of Y` and `Status: ...`
- ROADMAP.md: Look for phase directories in `.planning/phases/`
- PLAN.md: Parse XML task elements
- SUMMARY.md existence = plan complete

### Don't: Skip the Discovery Phase
GSD's discovery workflow produces DISCOVERY.md with confidence levels. Plans built without discovery often fail because Claude makes wrong assumptions. The planning commands should encourage discovery first.

---

## Critical Success Factors

1. **Fresh context per plan** - The whole point of the loop
2. **Verification at task AND plan level** - Catch failures early
3. **Clear failure signals** - Know exactly what failed and why
4. **STATE.md updates** - Loop must know where it is
5. **Atomic commits** - Revertable units of work

---

## Questions the Implementing Agent Should Answer

Before starting implementation, verify understanding:

1. How does `claude` CLI spawn a new session? (Check SDK docs)
2. How do we capture Claude's output to parse signals?
3. How do we pass the execution prompt + plan path to Claude?
4. How do we detect timeout/hang conditions?
5. How do we update STATE.md atomically (avoid corruption)?

---

## Testing the Implementation

### Manual Test Sequence
```bash
# 1. Setup GSD project
npx get-shit-done-cc
claude "/gsd:new-project"   # Create PROJECT.md
claude "/gsd:create-roadmap" # Create ROADMAP.md + phases

# 2. Test ralph status
ralph status  # Should show "Phase 1, not started"

# 3. Test single plan execution
ralph run     # Should execute first plan, show result

# 4. Test loop
ralph run --loop 3  # Should execute up to 3 plans

# 5. Test failure handling
# Create a plan with intentionally failing verification
# Verify ralph stops and reports correctly
```

### Edge Cases to Test
- Empty .planning/ directory
- Missing STATE.md
- Plan with no tasks
- Plan with only checkpoint tasks (should skip all)
- Verification command that hangs
- Claude returning garbage (no signal)

---

## Appendix: GSD File Structure Reference

GSD creates this structure in `.planning/`:

```
.planning/
├── PROJECT.md          # Project vision, requirements, constraints
├── ROADMAP.md          # Milestones → Phases structure
├── STATE.md            # Living memory: position, decisions, issues
├── ISSUES.md           # Deferred issues tracking
├── config.json         # GSD configuration
└── phases/
    ├── 01-foundation/
    │   ├── DISCOVERY.md      # Research findings (optional)
    │   ├── 01-01-PLAN.md     # First plan (2-3 tasks)
    │   ├── 01-01-SUMMARY.md  # After execution
    │   ├── 01-02-PLAN.md     # Second plan
    │   └── 01-02-SUMMARY.md
    └── 02-auth/
        └── ...
```

Ralph reads this structure and executes plans in order.
