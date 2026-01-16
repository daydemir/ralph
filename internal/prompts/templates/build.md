<context>
You are executing a PRD from the product backlog.

All context files are in .ralph/:
- prd.json - PRD backlog (find your assigned PRD here)
- codebase-map.md - Project structure and build commands
- progress.txt - Previous work and learnings
- fix_plan.md - Known issues to avoid
</context>

<task>
Execute the assigned PRD following this workflow:

1. SELECT PRD
   - If a specific PRD was assigned, work on that one
   - Otherwise, select from pending PRDs (where `passes: false`)
   - Choose based on priority and dependencies
   - Check `may_depend_on` - dependencies should be done first
   - Output: `SELECTED_PRD: <feature-id>`

2. WRITE TESTS FIRST
   - Write failing tests for the feature
   - Run tests to confirm they FAIL
   - If tests pass, feature may already exist - verify before proceeding

3. IMPLEMENT
   - Follow the steps in the PRD
   - Work in appropriate directory per codebase-map.md
   - NO placeholder or stub implementations

4. VERIFY
   - Run build and test commands from codebase-map.md
   - Tests from step 2 should now PASS
   - Fix any failures before proceeding

5. UPDATE ARTIFACTS
   - Set `passes: true` in prd.json
   - Append to progress.txt (what you did, learnings)
   - Update fix_plan.md if you found bugs

6. COMMIT
   For each repo where you made changes:
   ```bash
   cd <repo-path>
   git add -A && git commit -m "feat(<scope>): <description>"
   ```

7. END ITERATION
   After completing the PRD, output exactly:
   ```
   ###ITERATION_COMPLETE###
   ```
   Do NOT continue to another PRD. The orchestrator will start a fresh context for the next one.
</task>

<constraints>
- NO placeholder or stub implementations
- NO continuing to another PRD after completion
- Dependencies must be completed first
- Tests must pass before marking PRD complete
</constraints>

<rules>
TYPE SAFETY:
- No force unwraps unless truly impossible states
- Use Result types for fallible operations
- Typed structs, not dictionary parsing
- Exhaustive switch statements

FUNCTIONAL STYLE:
- Pure functions where possible
- map/filter/reduce over loops
- Immutable data by default

MINIMAL CODE:
- No overengineering
- No placeholder implementations
- Same input -> same output
</rules>
