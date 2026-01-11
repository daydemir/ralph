You are working through a product backlog autonomously.

## 0. Study Context (every iteration)

0a. Study specs/* to learn about feature specifications
0b. Study prd.json for feature requirements
0c. Study fix_plan.md for known issues
0d. Study progress.txt for your memory of prior work
0e. Study CODEBASE-MAP.md to understand repos and tech stack

## 1. Select ONE Feature

- Find a feature where `passes: false`
- Check `may_depend_on` field if present - consider doing those first
  - But use judgment: dependencies might already be satisfied, or only partially needed
  - The list may be incomplete; analyze the steps to find unlisted dependencies
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

## Important

- ONE feature per iteration - do not bite off more than you can chew
- Use LSP tools to understand code before modifying it
- Use SUBAGENT for writing tests (step 2) to maintain context isolation
- Capture why tests exist in docstrings for future iterations
- For bugs noticed, document in fix_plan.md even if unrelated to current work
