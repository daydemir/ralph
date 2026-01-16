# Deviation Rules Reference

While executing tasks, you WILL discover work not in the plan. This is normal. Apply these rules automatically and track all deviations for documentation.

## Rule 1: Auto-Fix Bugs

**Trigger:** Code doesn't work as intended (broken behavior, incorrect output, errors)

**Action:** Fix immediately, track for summary

**Examples:**
- Wrong SQL query returning incorrect data
- Logic errors (inverted condition, off-by-one, infinite loop)
- Type errors, null pointer exceptions, undefined references
- Broken validation (accepts invalid input, rejects valid input)
- Security vulnerabilities (SQL injection, XSS, CSRF, insecure auth)
- Race conditions, deadlocks
- Memory leaks, resource leaks

**Process:**
1. Fix the bug inline
2. Add/update tests to prevent regression
3. Verify fix works
4. Continue task
5. Record observation: `type="bug"` with `action="needs-fix"`

**No permission needed.** Bugs must be fixed for correct operation.

## Rule 2: Auto-Add Missing Critical Functionality

**Trigger:** Code is missing essential features for correctness, security, or basic operation

**Action:** Add immediately, track for summary

**Examples:**
- Missing error handling (no try/catch, unhandled promise rejections)
- No input validation (accepts malicious data, type coercion issues)
- Missing null/undefined checks (crashes on edge cases)
- No authentication on protected routes
- Missing authorization checks (users can access others' data)
- No CSRF protection, missing CORS configuration
- No rate limiting on public APIs
- Missing required database indexes (causes timeouts)
- No logging for errors (can't debug production)

**Process:**
1. Add the missing functionality inline
2. Add tests for the new functionality
3. Verify it works
4. Continue task
5. Record observation: `type="security"` or `type="missing-critical"`

**Critical = required for correct/secure/performant operation.** Not "features" - requirements for basic correctness.

## Rule 3: Auto-Fix Blocking Issues

**Trigger:** Something prevents you from completing current task

**Action:** Fix immediately to unblock, track for summary

**Examples:**
- Missing dependency (package not installed, import fails)
- Wrong types blocking compilation
- Broken import paths (file moved, wrong relative path)
- Missing environment variable (app won't start)
- Database connection config error
- Build configuration error (webpack, tsconfig, etc.)
- Missing file referenced in code
- Circular dependency blocking module resolution

**Process:**
1. Fix the blocking issue
2. Verify task can now proceed
3. Continue task
4. Record observation: `type="blocker"` with `action="needs-fix"`

**No permission needed.** Can't complete task without fixing blocker.

## Rule 4: Record Architectural Changes

**Trigger:** Fix/addition requires significant structural modification

**Action:** Record as observation, continue if safe, otherwise signal blocked

**Examples:**
- Adding new database table (not just column)
- Major schema changes (changing primary key, splitting tables)
- Introducing new service layer or architectural pattern
- Switching libraries/frameworks
- Changing authentication approach
- Adding new infrastructure (message queue, cache layer)
- Changing API contracts (breaking changes)

**Process for safe changes:**
1. Record observation with full context
2. Implement if it doesn't require user decision
3. Document the change thoroughly

**Process for changes needing approval:**
1. Record observation with full context
2. Signal `###BLOCKED:architectural_decision###`
3. Wait for human input

## Rule Priority

When multiple rules could apply:

1. **If Rule 4 applies and needs approval** → Signal blocked
2. **If Rules 1-3 apply** → Fix automatically, record observation
3. **If genuinely unsure** → Record observation, continue if safe

## Edge Case Guidance

| Situation | Rule | Why |
|-----------|------|-----|
| "This validation is missing" | Rule 2 | Critical for security |
| "This crashes on null" | Rule 1 | Bug |
| "Need to add table" | Rule 4 | Architectural |
| "Need to add column" | Rule 1 or 2 | Depends on purpose |
| "Package not installed" | Rule 3 | Blocking |
| "Wrong type annotation" | Rule 1 | Bug |
| "Missing auth middleware" | Rule 2 | Critical security |
| "Need to restructure modules" | Rule 4 | Architectural |

## Recording Deviations

Use XML observations to record all deviations:

```xml
<observation type="bug" severity="high">
  <title>Fixed case-sensitive email uniqueness</title>
  <detail>Email comparison was case-sensitive, allowing duplicate accounts with different casing. Fixed by normalizing to lowercase before comparison.</detail>
  <file>src/api/auth/signup.ts</file>
  <action>needs-fix</action>
</observation>
```

**Record AS YOU FIX** - don't batch at the end.

## Summary Documentation

After plan completion, the summary.json should document deviations:

```markdown
## Deviations from Plan

### Auto-Fixed Issues

**1. [Rule 1 - Bug] Fixed case-sensitive email uniqueness**
- Found during: Task 2
- Issue: Email comparison allowed duplicates with different casing
- Fix: Normalized to lowercase before comparison
- Files: src/api/auth/signup.ts
- Commit: abc123

**2. [Rule 3 - Blocking] Added missing bcrypt dependency**
- Found during: Task 1
- Issue: bcrypt not in package.json
- Fix: npm install bcrypt
- Files: package.json, package-lock.json
- Commit: def456
```

Or if none: "None - plan executed exactly as written."
