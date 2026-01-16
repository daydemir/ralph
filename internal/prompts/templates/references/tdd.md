# TDD Reference

TDD is about design quality, not coverage metrics. The red-green-refactor cycle forces you to think about behavior before implementation, producing cleaner interfaces and more testable code.

**Principle:** If you can describe the behavior as `expect(fn(input)).toBe(output)` before writing `fn`, TDD improves the result.

**Key insight:** TDD work is fundamentally heavier than standard tasks—it requires 2-3 execution cycles (RED → GREEN → REFACTOR), each with file reads, test runs, and potential debugging.

## When to Use TDD

### TDD Candidates (use `type: "tdd"` tasks)
- Business logic with defined inputs/outputs
- API endpoints with request/response contracts
- Data transformations, parsing, formatting
- Validation rules and constraints
- Algorithms with testable behavior
- State machines and workflows
- Utility functions with clear specifications

### Skip TDD (use `type: "auto"` tasks)
- UI layout, styling, visual components
- Configuration changes
- Glue code connecting existing components
- One-off scripts and migrations
- Simple CRUD with no business logic
- Exploratory prototyping

**Heuristic:** Can you write `expect(fn(input)).toBe(output)` before writing `fn`?
- Yes: Use `type: "tdd"`
- No: Use `type: "auto"`, add tests after if needed

## TDD Task Structure

```json
{
  "id": "task-1",
  "type": "tdd",
  "name": "Implement email validation",
  "behavior": "Valid emails accepted, invalid rejected, empty handled",
  "implementation": "Regex pattern matching RFC 5322",
  "files": ["src/lib/validation.ts", "src/lib/validation.test.ts"],
  "verify": "npm test -- --grep 'email validation'",
  "done": "All test cases pass",
  "status": "pending"
}
```

### TDD Task Fields

| Field | Description |
|-------|-------------|
| `type` | Must be `"tdd"` |
| `behavior` | Expected behavior in testable terms (input → output) |
| `implementation` | How to implement once tests pass |
| `files` | Both source file AND test file |
| `verify` | Test command that proves feature works |

## Red-Green-Refactor Cycle

### RED - Write failing test
1. Create test file following project conventions
2. Write test describing expected behavior (from `behavior` field)
3. Run test - it MUST fail
4. If test passes: feature exists or test is wrong. Investigate.
5. Commit: `test({phase}-{plan}): add failing test for [feature]`

### GREEN - Implement to pass
1. Write minimal code to make test pass
2. No cleverness, no optimization - just make it work
3. Run test - it MUST pass
4. Commit: `feat({phase}-{plan}): implement [feature]`

### REFACTOR (if needed)
1. Clean up implementation if obvious improvements exist
2. Run tests - MUST still pass
3. Only commit if changes made: `refactor({phase}-{plan}): clean up [feature]`

**Result:** Each TDD task produces 2-3 atomic commits.

## Test Quality Guidelines

### Test behavior, not implementation
- Good: "returns formatted date string"
- Bad: "calls formatDate helper with correct params"
- Tests should survive refactors

### One concept per test
- Good: Separate tests for valid input, empty input, malformed input
- Bad: Single test checking all edge cases with multiple assertions

### Descriptive names
- Good: "should reject empty email", "returns null for invalid ID"
- Bad: "test1", "handles error", "works correctly"

### No implementation details
- Good: Test public API, observable behavior
- Bad: Mock internals, test private methods, assert on internal state

## Framework Setup

When executing a TDD task but no test framework is configured, set it up as part of the RED phase:

| Project | Framework | Install |
|---------|-----------|---------|
| Node.js | Jest | `npm install -D jest @types/jest ts-jest` |
| Node.js (Vite) | Vitest | `npm install -D vitest` |
| Python | pytest | `pip install pytest` |
| Go | testing | Built-in |
| Rust | cargo test | Built-in |

## Error Handling

**Test doesn't fail in RED phase:**
- Feature may already exist - investigate
- Test may be wrong (not testing what you think)
- Fix before proceeding

**Test doesn't pass in GREEN phase:**
- Debug implementation
- Don't skip to refactor
- Keep iterating until green

**Tests fail in REFACTOR phase:**
- Undo refactor
- Commit was premature
- Refactor in smaller steps

**Unrelated tests break:**
- Stop and investigate
- May indicate coupling issue
- Fix before proceeding

## Commit Pattern

TDD tasks produce 2-3 atomic commits (one per phase):

```
test(08-02): add failing test for email validation

- Tests valid email formats accepted
- Tests invalid formats rejected
- Tests empty input handling

feat(08-02): implement email validation

- Regex pattern matches RFC 5322
- Returns boolean for validity
- Handles edge cases (empty, null)

refactor(08-02): extract regex to constant (optional)

- Moved pattern to EMAIL_REGEX constant
- No behavior changes
- Tests still pass
```

## Context Budget

TDD tasks target **~40% context usage** (lower than standard ~50%).

Why lower:
- RED phase: write test, run test, potentially debug why it didn't fail
- GREEN phase: implement, run test, potentially iterate on failures
- REFACTOR phase: modify code, run tests, verify no regressions

Each phase involves reading files, running commands, analyzing output. The back-and-forth is inherently heavier than linear task execution.

## Example TDD Plan

```json
{
  "phase": "02-validation",
  "planNumber": "01",
  "status": "pending",
  "objective": "Implement input validation with TDD",
  "tasks": [
    {
      "id": "task-1",
      "type": "tdd",
      "name": "Implement email validation",
      "behavior": "Valid emails return true, invalid return false, empty throws",
      "implementation": "RFC 5322 regex pattern with empty check",
      "files": ["src/lib/validators.ts", "src/lib/validators.test.ts"],
      "verify": "npm test -- validators",
      "done": "All email validation tests pass",
      "status": "pending"
    },
    {
      "id": "task-2",
      "type": "tdd",
      "name": "Implement password strength validation",
      "behavior": "Passwords must be 8+ chars, have upper/lower/number",
      "implementation": "Check length and character class requirements",
      "files": ["src/lib/validators.ts", "src/lib/validators.test.ts"],
      "verify": "npm test -- validators",
      "done": "All password validation tests pass",
      "status": "pending"
    }
  ],
  "verification": ["npm test", "npm run build"],
  "createdAt": "2026-01-16T10:00:00Z",
  "completedAt": null
}
```
