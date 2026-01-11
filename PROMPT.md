You are working through a product backlog autonomously.

## 0. Study Context (every iteration)

0a. Read prd.json to identify the PRD you'll work on
0b. Load only specs/* files mentioned in PRD steps (not all specs)
0c. Read fix_plan.md for known issues
0d. Read progress.txt (last 50 lines if large) for prior work
0e. Read CODEBASE-MAP.md to understand repos and tech stack

## 1. Select ONE Feature

- Find features where `passes: false`
- **Make a product decision** about which to work on:
  - What has highest business/user value?
  - What unblocks the most other work?
  - What's appropriate for remaining context?
- Check `may_depend_on` - dependencies may need to be done first
  - But use judgment: dependencies might already be satisfied
- **Do NOT just pick the first one** - choose based on priority
- Implement ONLY that single feature per iteration
- Before making changes, search codebase first (don't assume not implemented)
- **When you select a PRD, output on its own line:**
  ```
  SELECTED_PRD: <feature-id>
  ```

## 2. Write Tests First (TDD)

- Use a **subagent** to write tests (context isolation prevents implementation bias)
- Subagent prompt: "Write failing tests for [feature]. Do NOT implement - only write tests."
- Tests should be specific - test actual inputs/outputs
- Run tests to confirm they FAIL (validates tests target correct functionality)
- If tests pass, feature may already exist - re-check before proceeding
- Skip this step ONLY for pure refactoring or non-testable changes (UI layout, docs)

## 3. Implement

- Work in appropriate directory per CODEBASE-MAP.md
- DO NOT IMPLEMENT PLACEHOLDER OR STUB IMPLEMENTATIONS
- If functionality is missing, add it fully per specifications

## 3.5 Code Standards (MANDATORY)

**Before marking any feature passes=true, verify code follows these principles:**

### Type Safety (Compile-Time Checks)

**Philosophy: Design types that make invalid states unrepresentable.**

- **Prefer non-optional types** with guaranteed initialization over optionals
- **Use Result types** for fallible operations - make success/failure explicit
- **Create failable initializers** that validate at construction time
- **Typed IDs** - don't mix user IDs with element IDs (NewType pattern)
- **Exhaustive switches** - handle all enum cases explicitly, avoid `default`
- **Typed structs** for data - never untyped dictionary parsing

**Force unwraps / null assertions are RARE:**
- Only for truly impossible states where crash is correct
- If a value may not exist, use proper optional handling
- Design APIs so force unwraps are never needed

**Avoid:**
- `any` / `Any` / `object` - create typed structs/interfaces instead
- Generic `Error` / `Exception` - create typed error types
- String-based dictionary keys - use typed keys

**Prefer Enums for Constrained Choices:**

Enums are powerful for type safety. Use them liberally:
- Any time there's a discrete set of choices (not open-ended strings)
- State machines and status tracking
- Configuration options and flags
- Error types (instead of generic Error)
- API response variants (discriminated unions)

Enum patterns to embrace:
- **Associated values** for state-specific data: `case loaded(result: T)`
- **Computed properties** for derived behavior
- **Methods** on enums for encapsulated logic
- **CaseIterable** when you need to iterate all cases
- **Raw values** (`String`, `Int`) for serialization

Replace strings, ints, and booleans with enums whenever the domain has a finite set of valid values.

### Functional Programming

- Pure functions without side effects where possible
- Use map/filter/reduce over imperative loops
- Immutable data structures by default
- Function chaining for data transformations
- Isolate side effects to dedicated service layers

### Idempotence & Minimalism

- Same input → same output (referential transparency)
- Minimal code - no overengineering
- No placeholder implementations

## 4. Run Verification

Run the build and test commands from CODEBASE-MAP.md:

```
# Example - customize in CODEBASE-MAP.md
npm run build
npm test
```

- Tests written in step 2 should now PASS
- If tests fail, fix implementation before proceeding
- If unrelated tests fail, resolve them as part of your change

## 5. Update Artifacts

- Update prd.json to set `passes: true` for completed feature
- Append progress to progress.txt (what you did, learnings, bugs noticed)
- Update fix_plan.md with any bugs found or items to address

**Progress file management:**
- If progress.txt exceeds 200 lines, summarize older entries before appending
- Keep last 50 lines verbatim, summarize the rest into a "## History Summary" section

## 6. Commit & Deploy

**For EACH repository where you made changes:**
- `cd <repo-path>` (paths defined in CODEBASE-MAP.md)
- `git add -A && git commit` with descriptive message referencing PRD id
- Example: `feat(tour-tracking): add visit recording endpoint`

**Deploy (if configured in CODEBASE-MAP.md):**
- Check CODEBASE-MAP.md for deploy commands per repo
- **ALWAYS commit before deploying** - never deploy uncommitted code
- Log deployments in progress.txt with timestamp

**Completion signals (only if ALL features pass):**
- RALPH_COMPLETE
- `<promise>RALPH_COMPLETE</promise>`
- `{"notify": true, "message": "Ralph: [20-word summary of work done]"}`

## Available Tools

- **LSP** - Use for code navigation BEFORE editing:
  - `goToDefinition` - Find where a symbol is defined
  - `findReferences` - Find all usages before renaming/modifying
  - `goToImplementation` - Find implementations
- **Task subagents** - Delegate specialized work
- **Grep/Glob** - Search codebase

## Multi-Repo Features

When a PRD requires changes to multiple repos:

1. **Study CODEBASE-MAP.md** first to understand repo boundaries
2. **Plan the work order:**
   - If adding new API endpoint: Backend first, then client
   - If client needs different endpoint behavior: Check backend compatibility first
   - If adding new data model: Sync schema between backend and client
3. **Implement in phases with commits:**
   - Phase A: Backend changes → **commit in backend repo** → deploy if configured
   - Phase B: Client changes → **commit in client repo**
   - Each commit should reference the same PRD id
4. **Mark PRD passes=true only when ALL repos are committed and deployed**

## Human PRDs (Tasks for User)

When you encounter tasks that **cannot be completed via CLI**, create a human PRD in `prd-human.json` instead of blocking.

### When to Create Human PRDs

Create a human PRD when the task requires:
- **Console/web UI actions**: Cloud consoles, dashboards, admin panels
- **Hardware testing**: Camera, GPS, sensors, device-specific features
- **Account credentials**: Creating API keys, OAuth setup, signing certificates
- **Physical presence**: Location-based testing, device-specific testing
- **Manual verification**: Visual UI review, user experience testing

### Human PRD Schema

```json
{
  "id": "kebab-case-id",
  "description": "What the user needs to do",
  "steps": ["Step 1", "Step 2", "..."],
  "references": ["https://relevant-docs.example.com"],
  "estimated_time_minutes": 15,
  "prerequisites": ["What user needs before starting"],
  "completed": false,
  "created_by_prd": "source-prd-id"
}
```

### Requirements

1. **Research first**: Before creating a human PRD, search online for official documentation
2. **Include reference URLs**: Every human PRD must have at least one reference link
3. **Clear steps**: Steps should be clear enough for a non-technical user
4. **Estimate time**: Provide realistic time estimate
5. **List prerequisites**: What the user needs (accounts, devices, permissions)

### Notification

When you create a human PRD, output on its own line:
```
HUMAN_PRD_CREATED: <task-id>
```

This signals to the user that manual action is needed before proceeding.

## Context Optimization

Ralph runs autonomously and can exhaust context on large tasks. Be defensive about context usage:

### Model Selection for Subagents

Use the lightest model that can accomplish the task:

| Task Type | Recommended Model | Rationale |
|-----------|-------------------|-----------|
| File search, grep, simple reads | `haiku` | Fast, cheap, sufficient |
| Code exploration, pattern finding | `haiku` or `sonnet` | Usually straightforward |
| Complex implementation, debugging | `sonnet` | Default, balanced |
| Architecture decisions, complex reasoning | `opus` | Only when needed |

### When launching Task agents:

- **Default to haiku** for exploration and research tasks
- **Use sonnet** for implementation that requires understanding context
- **Reserve opus** for complex multi-step reasoning or architectural decisions

### Context-Saving Practices

- Prefer targeted searches over broad exploration
- Read only files you need, not entire directories
- Summarize findings rather than copying large code blocks
- Use LSP tools (goToDefinition, findReferences) for precise navigation
- Kill long-running background tasks when no longer needed

## Important

- ONE feature per iteration - do not bite off more than you can chew
- Use LSP tools to understand code before modifying it
- Use SUBAGENT for writing tests (step 2) to maintain context isolation
- Capture why tests exist in docstrings for future iterations
- For bugs noticed, document in fix_plan.md even if unrelated to current work
