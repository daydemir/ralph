# Ralph Task Format Reference

Ralph uses JSON-based plans with structured tasks. This document defines the task format and types.

## Plan JSON Structure

```json
{
  "phase": "01-foundation",
  "planNumber": "01",
  "status": "pending",
  "objective": "Clear description of what this plan accomplishes",
  "tasks": [...],
  "verification": ["npm run build", "npm test"],
  "createdAt": "2026-01-16T...",
  "completedAt": null
}
```

## Task Structure

Each task in the `tasks` array:

```json
{
  "id": "task-1",
  "name": "Create user authentication endpoint",
  "type": "auto",
  "files": ["src/api/auth.ts", "src/types/user.ts"],
  "action": "Detailed implementation instructions...",
  "verify": "npm test -- auth.test.ts",
  "done": "Login endpoint returns JWT token on valid credentials",
  "status": "pending"
}
```

## Task Types (2 types only)

### `auto` - Fully Autonomous
Claude can complete without human intervention.

**Required fields:** id, name, type, files, action, verify, done, status

**Example:**
```json
{
  "id": "task-1",
  "name": "Create login API endpoint",
  "type": "auto",
  "files": ["src/app/api/auth/login/route.ts"],
  "action": "Create POST endpoint accepting {email, password}. Validate with bcrypt against User table. Return JWT in httpOnly cookie with 15-min expiry. Use jose library.",
  "verify": "curl -X POST localhost:3000/api/auth/login -d '{\"email\":\"test@test.com\",\"password\":\"password\"}' returns 200 with Set-Cookie header",
  "done": "Valid credentials return 200 + JWT cookie, invalid return 401",
  "status": "pending"
}
```

### `manual` - Requires Human Action
Task cannot be automated, requires human intervention.

**Use for all scenarios where human input is needed:**
- Visual verification (UI checks, layout, styling)
- Implementation decisions (technology selection, architecture choices)
- External actions (email verification, SMS codes)
- Physical device interaction

**Example - Visual Verification:**
```json
{
  "id": "task-2",
  "name": "Verify dashboard layout",
  "type": "manual",
  "files": [],
  "action": "Visit http://localhost:3000/dashboard and verify: sidebar visible on desktop, collapses on mobile, no layout shifts.",
  "verify": "User confirms dashboard displays correctly",
  "done": "Dashboard layout approved",
  "status": "pending"
}
```

**Example - Implementation Decision:**
```json
{
  "id": "task-3",
  "name": "Select authentication provider",
  "type": "manual",
  "files": [],
  "action": "Choose between Supabase Auth, Clerk, or NextAuth based on project requirements. Options: (1) Supabase Auth - Built-in with DB, less customizable. (2) Clerk - Best DX, paid after 10k MAU. (3) NextAuth - Free, flexible, more setup.",
  "verify": "Decision documented",
  "done": "Authentication provider selected and documented",
  "status": "pending"
}
```

**Example - External Action:**
```json
{
  "id": "task-4",
  "name": "Verify email authentication link",
  "type": "manual",
  "files": [],
  "action": "Click the verification link sent to the user's email. This cannot be automated as it requires accessing an external email inbox.",
  "verify": "User's email_verified field is true in database",
  "done": "Email verification complete",
  "status": "pending"
}
```

## Task Status Values

| Status | Meaning |
|--------|---------|
| `pending` | Not yet started |
| `in_progress` | Currently being executed |
| `complete` | Successfully completed |
| `failed` | Failed (covers skipped/blocked cases) |

## Task Field Requirements

| Field | auto | manual |
|-------|------|--------|
| id | Required | Required |
| name | Required | Required |
| type | Required | Required |
| files | Required | Optional |
| action | Required | Required |
| verify | Required | Optional |
| done | Required | Required |
| status | Required | Required |

## Writing Good Tasks

### Action Field
**Good:**
```
Create POST endpoint accepting {email, password}. Validate using bcrypt
against User table. Return JWT in httpOnly cookie with 15-min expiry.
Use jose library (not jsonwebtoken - CommonJS issues with Edge runtime).
```

**Bad:**
```
Add authentication
```

### Verify Field
**Good:**
```
curl -X POST /api/auth/login -d '{"email":"test@test.com","password":"pass"}'
returns 200 with Set-Cookie header containing JWT
```

**Bad:**
```
It works
```

### Done Field
**Good:**
```
Valid credentials return 200 + JWT cookie, invalid credentials return 401,
missing fields return 400 with validation errors
```

**Bad:**
```
Authentication is complete
```

## Task Sizing Guidelines

Each task should take Claude **15-60 minutes** to execute:

| Duration | Action |
|----------|--------|
| < 15 min | Too small - combine with related task |
| 15-60 min | Right size - single focused unit |
| > 60 min | Too large - split into smaller tasks |

**Signals task is too large:**
- Touches more than 3-5 files
- Has multiple distinct "chunks" of work
- The action section is more than a paragraph

**Signals tasks should combine:**
- One task just sets up for the next
- Separate tasks touch the same file
- Neither task is meaningful alone

## Invalid Task Types

The following are **NOT valid** task types and will cause validation errors:
- `checkpoint:human-verify`
- `checkpoint:human-action`
- `checkpoint:decision`

All checkpoint scenarios should use `type: "manual"` with a descriptive action field.

## Example Complete Plan

```json
{
  "phase": "02-authentication",
  "planNumber": "01",
  "status": "pending",
  "objective": "Implement JWT-based authentication with login and logout endpoints",
  "tasks": [
    {
      "id": "task-1",
      "name": "Create User model and schema",
      "type": "auto",
      "files": ["prisma/schema.prisma", "src/types/user.ts"],
      "action": "Add User model to Prisma schema with id (UUID), email (unique), passwordHash, createdAt, updatedAt. Generate TypeScript types.",
      "verify": "npx prisma db push succeeds, generated types exist",
      "done": "User model in database, TypeScript types available",
      "status": "pending"
    },
    {
      "id": "task-2",
      "name": "Create login endpoint",
      "type": "auto",
      "files": ["src/app/api/auth/login/route.ts", "src/lib/auth.ts"],
      "action": "POST /api/auth/login accepting {email, password}. Validate with bcrypt. Return JWT (jose library) in httpOnly cookie. 15-min expiry.",
      "verify": "curl test with valid/invalid credentials returns expected responses",
      "done": "Valid: 200 + cookie. Invalid: 401. Missing fields: 400.",
      "status": "pending"
    },
    {
      "id": "task-3",
      "name": "Create logout endpoint",
      "type": "auto",
      "files": ["src/app/api/auth/logout/route.ts"],
      "action": "POST /api/auth/logout clears the auth cookie by setting empty value with past expiry.",
      "verify": "curl test shows cookie cleared",
      "done": "Cookie cleared, subsequent requests are unauthenticated",
      "status": "pending"
    }
  ],
  "verification": [
    "npm run build passes",
    "npm test passes",
    "Manual test: login -> access protected route -> logout -> cannot access"
  ],
  "createdAt": "2026-01-16T10:00:00Z",
  "completedAt": null
}
```
