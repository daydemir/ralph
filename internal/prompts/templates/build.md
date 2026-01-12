You are executing a PRD from the product backlog.

## Context Files

All files are in .ralph/:
- prd.json - PRD backlog (find your assigned PRD here)
- codebase-map.md - Project structure and build commands
- progress.txt - Previous work and learnings
- fix_plan.md - Known issues to avoid

## Workflow

### 1. Select PRD

If a specific PRD was assigned, work on that one.
Otherwise, select from pending PRDs (where `passes: false`):
- Choose based on priority and dependencies
- Check `may_depend_on` - dependencies should be done first

**Output your selection:**
```
SELECTED_PRD: <feature-id>
```

### 2. Write Tests First

- Write failing tests for the feature
- Run tests to confirm they FAIL
- If tests pass, feature may already exist - verify before proceeding

### 3. Implement

- Follow the steps in the PRD
- Work in appropriate directory per codebase-map.md
- NO placeholder or stub implementations

### 4. Verify

Run build and test commands from codebase-map.md:
- Tests from step 2 should now PASS
- Fix any failures before proceeding

### 5. Update Artifacts

- Set `passes: true` in prd.json
- Append to progress.txt (what you did, learnings)
- Update fix_plan.md if you found bugs

### 6. Commit

For each repo where you made changes:
```bash
cd <repo-path>
git add -A && git commit -m "feat(<scope>): <description>"
```

### 7. End Iteration

After completing the PRD, output exactly:
```
###ITERATION_COMPLETE###
```

Do NOT continue to another PRD. The orchestrator will start a fresh context for the next one.

## Code Standards

### Type Safety
- No force unwraps unless truly impossible states
- Use Result types for fallible operations
- Typed structs, not dictionary parsing
- Exhaustive switch statements

### Functional Style
- Pure functions where possible
- map/filter/reduce over loops
- Immutable data by default

### Minimal Code
- No overengineering
- No placeholder implementations
- Same input → same output
