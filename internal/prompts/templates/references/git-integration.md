# Git Integration

Git integration for ralph-managed projects.

## Core Principle

**Commit outcomes, not process.**

The git log should read like a changelog of what shipped, not a diary of planning activity.

## Commit Points

| Event | Commit? | Why |
|-------|---------|-----|
| project.json + roadmap.json created | YES | Project initialization |
| plan.json created | NO | Intermediate - commit with plan completion |
| research.json created | NO | Intermediate |
| **Task completed** | YES | Atomic unit of work (1 commit per task) |
| **Plan completed** | YES | Metadata (summary.json + state.json update) |
| Handoff created | YES | WIP state preserved |

## Git Check

```bash
[ -d .git ] && echo "GIT_EXISTS" || echo "NO_GIT"
```

If NO_GIT: Run `git init` silently. Ralph projects always get their own repo.

## Commit Formats

### Project Initialization

When project.json and roadmap.json are created together:

```
docs: initialize [project-name] ([N] phases)

[One-liner from project.json description]

Phases:
1. [phase-name]: [goal]
2. [phase-name]: [goal]
3. [phase-name]: [goal]
```

What to commit:
```bash
git add .ralph/
git commit
```

### Task Completion

Each task gets its own commit immediately after completion.

```
{type}({phase}-{plan}): {task-name}

- [Key change 1]
- [Key change 2]
- [Key change 3]
```

**Commit types:**

| Type | Use for |
|------|---------|
| `feat` | New feature/functionality |
| `fix` | Bug fix |
| `test` | Test-only (TDD RED phase) |
| `refactor` | Code cleanup (TDD REFACTOR phase) |
| `perf` | Performance improvement |
| `chore` | Dependencies, config, tooling |

**Examples:**

```bash
# Standard task
git add src/api/auth.ts src/types/user.ts
git commit -m "feat(08-02): create user registration endpoint

- POST /auth/register validates email and password
- Checks for duplicate users
- Returns JWT token on success
"

# TDD task - RED phase
git add src/__tests__/jwt.test.ts
git commit -m "test(07-02): add failing test for JWT generation

- Tests token contains user ID claim
- Tests token expires in 1 hour
- Tests signature verification
"

# TDD task - GREEN phase
git add src/utils/jwt.ts
git commit -m "feat(07-02): implement JWT generation

- Uses jose library for signing
- Includes user ID and expiry claims
- Signs with HS256 algorithm
"
```

### Plan Completion

After all tasks committed, one final metadata commit captures plan completion.

```
docs({phase}-{plan}): complete [plan-name] plan

Tasks completed: [N]/[N]
- [Task 1 name]
- [Task 2 name]
- [Task 3 name]

SUMMARY: .ralph/phases/XX-name/{phase}-{plan}-summary.json
```

What to commit:
```bash
git add .ralph/phases/XX-name/{phase}-{plan}-plan.json
git add .ralph/phases/XX-name/{phase}-{plan}-summary.json
git add .ralph/state.json
git add .ralph/roadmap.json
git commit
```

**Note:** Code files NOT included - already committed per-task.

### Handoff (WIP)

```
wip: [phase-name] paused at task [X]/[Y]

Current: [task name]
[If blocked:] Blocked: [reason]
```

What to commit:
```bash
git add .ralph/
git commit
```

## Example Git Log

**Per-task commits (recommended):**
```
# Phase 04 - Checkout
1a2b3c docs(04-01): complete checkout flow plan
4d5e6f feat(04-01): add webhook signature verification
7g8h9i feat(04-01): implement payment session creation
0j1k2l feat(04-01): create checkout page component

# Phase 03 - Products
3m4n5o docs(03-02): complete product listing plan
6p7q8r feat(03-02): add pagination controls
9s0t1u feat(03-02): implement search and filters
2v3w4x feat(03-01): create product catalog schema

# Phase 02 - Auth
5y6z7a docs(02-02): complete token refresh plan
8b9c0d feat(02-02): implement refresh token rotation
1e2f3g test(02-02): add failing test for token refresh
4h5i6j docs(02-01): complete JWT setup plan
7k8l9m feat(02-01): add JWT generation and validation
0n1o2p chore(02-01): install jose library

# Phase 01 - Foundation
3q4r5s docs(01-01): complete scaffold plan
6t7u8v feat(01-01): configure Tailwind and globals
9w0x1y feat(01-01): set up Prisma with database
2z3a4b feat(01-01): create Next.js 15 project

# Initialization
5c6d7e docs: initialize ecommerce-app (5 phases)
```

Each plan produces 2-4 commits (tasks + metadata). Clear, granular, bisectable.

## Anti-Patterns

**Don't commit (intermediate artifacts):**
- plan.json creation (commit with plan completion)
- research.json (intermediate)
- Minor planning tweaks
- "Fixed typo in roadmap"

**Do commit (outcomes):**
- Each task completion (feat/fix/test/refactor)
- Plan completion metadata (docs)
- Project initialization (docs)

**Key principle:** Commit working code and shipped outcomes, not planning process.

## Rationale

### Context engineering for AI
- Git history becomes primary context source for future Claude sessions
- `git log --grep="{phase}-{plan}"` shows all work for a plan
- `git diff <hash>^..<hash>` shows exact changes per task
- Less reliance on parsing summary.json = more context for actual work

### Failure recovery
- Task 1 committed, Task 2 failed
- Next session: sees task 1 complete, can retry task 2
- Can `git reset --hard` to last successful task

### Debugging
- `git bisect` finds exact failing task, not just failing plan
- `git blame` traces line to specific task context
- Each commit is independently revertable

### Observability
- Solo developer + Claude workflow benefits from granular attribution
- Atomic commits are git best practice
- "Commit noise" irrelevant when consumer is Claude, not humans
