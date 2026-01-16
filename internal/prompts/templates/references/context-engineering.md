# Context Engineering Reference

Context is your most precious resource. Quality degrades predictably as context fills. This document provides strategies for effective context management.

## Quality Degradation Curve

| Context Usage | Quality | Claude's State |
|---------------|---------|----------------|
| 0-30% | PEAK | Thorough, comprehensive |
| 30-50% | GOOD | Confident, solid work |
| 50-70% | DEGRADING | Efficiency mode begins |
| 70%+ | POOR | Rushed, minimal |

**The rule:** Stop BEFORE quality degrades. Plans should complete within ~50% context.

## Self-Monitoring Heuristics

Track your own context usage:

1. **Tool call count:** If > 50 tool calls without task completion, you're burning context
2. **Repeated errors:** 3+ retries of same fix = stuck, bail out
3. **File reading volume:** If you've read > 20 files without progress, context is bloated
4. **Output length:** Long responses burn context faster

## Context-Saving Techniques

### Use Subagents for Writing

Instead of editing files directly (burns your context), delegate to subagents:

```
Task(subagent_type="general-purpose", prompt="
  Update PLAN.json Progress section to mark task-1 as complete.
  Add observation about test stubs found in tests/auth.test.ts.
")
```

**Benefits:**
- Subagent has fresh context for file editing
- Your main context preserved for execution logic
- Multiple edits batched into one delegation

### Read Only What You Need

**Bad:**
```
Read the entire file
Read all related files
Read the whole directory
```

**Good:**
```
Read specific function (lines 45-80)
Grep for the pattern you need
Read only files you're modifying
```

### Batch Related Operations

**Bad:** Read file, make change, read again, verify, read again...

**Good:** Read file, plan all changes, make all changes, verify once

### Progressive Loading

Don't load all context upfront. Load as needed:

1. Start with plan and critical files only
2. Load additional files when you actually need them
3. Don't re-read files you've already read

## When to Bail Out

**At ~100K tokens (50-60%), proactively bail out:**

1. Update Progress section with current state
2. Update Observations section with findings
3. Document what worked, what failed, next steps
4. Signal: `###BAILOUT:context_preservation###`

**Ralph will:**
- Recognize the bailout signal
- Mark as soft failure (not hard failure)
- Start fresh context for continuation
- Your progress is preserved in the plan file

## Checkpoint vs Bailout

| Situation | Use |
|-----------|-----|
| Manual task requiring human input | Checkpoint (deferred) |
| Running low on context | Bailout |
| Work partially complete, need fresh context | Bailout |
| Build/test continuously failing | Failure signal |
| External dependency blocking | Blocked signal |

## Plan-Level Context Budget

Each plan targets ~50% context:

| Task Complexity | Tasks/Plan | Context/Task | Total |
|-----------------|------------|--------------|-------|
| Simple (CRUD, config) | 3 | ~10-15% | ~30-45% |
| Complex (auth, payments) | 2 | ~20-30% | ~40-50% |
| Very complex (migrations) | 1-2 | ~30-40% | ~30-50% |

## Recording Observations Efficiently

**Inline recording burns context.** Use this pattern:

```
Task(subagent_type="general-purpose", prompt="
  Add this observation to the plan's Observations section:
  <observation type=\"stub\" severity=\"medium\">
    <title>3 backend tests are stubs</title>
    <detail>image.test.ts and video.test.ts have stub tests</detail>
    <file>backend/src/__tests__/</file>
    <action>needs-implementation</action>
  </observation>
")
```

**Record AS YOU GO** - don't batch at the end when you might run out of context.

## Fresh Context Benefits

When Ralph starts a fresh context for continuation:

1. **Clean slate:** No accumulated confusion
2. **Progress preserved:** Plan file has current state
3. **Observations intact:** Analysis can use them
4. **Faster execution:** Not wading through old context

**Don't fear bailout.** It's a feature, not a failure.

## Anti-Patterns

### Over-Reading
```
# BAD: Read everything to "understand"
Read all 50 files in src/
Read all documentation
Read all tests
```

```
# GOOD: Read what you need
Read the 3 files you're modifying
Grep for the function you need
Read tests only when writing tests
```

### Verbose Output
```
# BAD: Long explanations of what you're doing
"I will now proceed to read the authentication module to understand..."
"Let me analyze the current implementation and consider..."
```

```
# GOOD: Terse, action-oriented
Reading auth module.
Found issue: missing validation.
```

### Redundant Operations
```
# BAD: Multiple reads of same file
Read file -> plan -> read again -> implement -> read again -> verify
```

```
# GOOD: Single comprehensive read
Read file -> plan + implement + verify in one pass
```

## Context Recovery

If you realize you're in degraded mode (70%+):

1. **Stop** - Don't push through
2. **Update Progress** - Save current state
3. **Record Observations** - What you learned
4. **Bailout** - Signal for fresh context

**Signs of degraded mode:**
- Responses getting shorter
- Skipping verification steps
- Making assumptions instead of checking
- Feeling "rushed" to complete
