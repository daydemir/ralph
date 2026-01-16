# Ralph Signals Reference

Ralph uses signals to communicate state between the executor agent and the orchestration loop.

## Completion Signals

### ###PLAN_COMPLETE###
Emit when ALL conditions are met:
- All tasks in the plan executed successfully
- All verification checks passed
- Build and tests pass (no new failures introduced)
- summary.json file created
- Background tasks finished and verified

**When to use:** Only after thorough verification. Ralph will verify summary.json exists before accepting this signal.

### ###BAILOUT:{reason}###
Emit when you need to preserve context and allow continuation:
- Context is getting high (~100K tokens)
- Work is partially complete but safe to pause
- Progress section has been updated

**Before emitting:**
1. Update the plan's Progress section with current state
2. Update Observations section with any findings
3. Document what worked, what failed, next steps

**Examples:**
- `###BAILOUT:context_preservation###` - Token limit approaching
- `###BAILOUT:partial_completion###` - Some tasks done, need fresh context
- `###BAILOUT:external_dependency###` - Waiting on something external

## Failure Signals

### ###TASK_FAILED:{task_name}###
A specific task could not be completed.

**When to use:** Task verification failed after attempts to fix, or task is fundamentally blocked.

### ###PLAN_FAILED:{check}###
Plan-level verification failed.

**When to use:** Overall verification checks from `<verification>` section failed.

### ###BUILD_FAILED:{project}###
Project build failed.

**When to use:** Build command returns non-zero, compilation errors exist.

**Examples:**
- `###BUILD_FAILED:ios###`
- `###BUILD_FAILED:backend###`
- `###BUILD_FAILED:frontend###`

### ###TEST_FAILED:{project}:{count}###
Tests failed that weren't failing before your changes.

**When to use:** New test failures introduced by your work.

**Examples:**
- `###TEST_FAILED:ios:3###` - 3 new test failures in iOS project
- `###TEST_FAILED:backend:1###` - 1 new test failure in backend

### ###BLOCKED:{reason}###
Execution cannot continue, needs human intervention.

**When to use:**
- Authentication required that can't be automated
- External service is down
- Architectural decision needed
- Access permissions required

**Examples:**
- `###BLOCKED:needs_api_key###`
- `###BLOCKED:auth_required###`
- `###BLOCKED:architectural_decision###`
- `###BLOCKED:external_service_unavailable###`

## Signal Placement

Signals should be emitted:
1. **After** completing all relevant work
2. **After** creating required artifacts (summary.json, Progress updates)
3. **At the end** of your response
4. **On a line by itself** for easy parsing

## Signal Priority

If multiple signals could apply, emit the most specific one:
1. `BLOCKED` - if you literally cannot proceed
2. `*_FAILED` - if something specific failed
3. `BAILOUT` - if preserving partial progress
4. `PLAN_COMPLETE` - only if truly complete

## What Happens After Signals

| Signal | Ralph's Response |
|--------|-----------------|
| `PLAN_COMPLETE` | Verifies summary.json, marks plan complete, proceeds to next |
| `BAILOUT` | Marks as soft failure, may retry with fresh context |
| `*_FAILED` | Marks as hard failure, stops loop, reports error |
| `BLOCKED` | Validates blocker claim, may reject or escalate |

## Observation Format (Not Signals)

Observations are recorded for analysis but don't control execution flow:

```xml
<observation type="TYPE" severity="SEVERITY">
  <title>Short descriptive title</title>
  <detail>What you found and why it matters</detail>
  <file>path/to/relevant/file</file>
  <action>ACTION</action>
</observation>
```

**Types:** bug, stub, api-issue, insight, blocker, technical-debt, assumption, scope-creep, dependency, questionable, already-complete, checkpoint-automated, tooling-friction, test-failed, test-infrastructure, manual-checkpoint-deferred

**Severities:** critical, high, medium, low, info

**Actions:** needs-fix, needs-implementation, needs-plan, needs-investigation, needs-documentation, needs-human-verify, none
