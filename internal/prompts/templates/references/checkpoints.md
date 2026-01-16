# Manual Task Patterns

Plans execute autonomously. Manual tasks formalize interaction points where human verification or decisions are needed.

**Core principle:** Claude automates everything with CLI/API. Manual tasks are for verification and decisions, not work Claude can automate.

## When to Use Manual Tasks

Use `type: "manual"` when human input is genuinely required:

### Visual/Functional Verification (most common)

**When:** Claude completed automated work, human confirms it works correctly.

**Use for:**
- Visual UI checks (layout, styling, responsiveness)
- Interactive flows (click through wizard, test user flows)
- Functional verification (feature works as expected)
- Audio/video playback quality
- Animation smoothness
- Accessibility testing

**Example:**
```json
{
  "id": "verify-1",
  "name": "Verify responsive dashboard",
  "type": "manual",
  "files": [],
  "action": "Visit http://localhost:3000/dashboard. Desktop: sidebar visible. Mobile: hamburger menu. No layout shifts.",
  "verify": "User confirms dashboard displays correctly",
  "done": "Dashboard layout approved",
  "status": "pending"
}
```

### Implementation Decisions

**When:** Human must make choice that affects implementation direction.

**Use for:**
- Technology selection (which auth provider, which database)
- Architecture decisions (monorepo vs separate repos)
- Design choices (color scheme, layout approach)
- Feature prioritization

**Example:**
```json
{
  "id": "decide-1",
  "name": "Select authentication provider",
  "type": "manual",
  "files": [],
  "action": "Choose between: (1) Supabase Auth - Built-in with DB, less customizable. (2) Clerk - Best DX, paid after 10k MAU. (3) NextAuth - Free, flexible, more setup.",
  "verify": "Decision documented in project.json",
  "done": "Authentication provider selected",
  "status": "pending"
}
```

### External Actions (rare)

**When:** Action has NO CLI/API and requires human-only interaction.

**Use ONLY for:**
- Email verification links (account creation requires clicking email)
- SMS 2FA codes (phone verification)
- Manual account approvals (platform requires human review)
- Credit card 3D Secure flows (web-based payment authorization)

**Do NOT use for:**
- Deploying (use CLI: `vercel`, `railway`, `fly`)
- Creating webhooks (use API)
- Creating databases (use provider CLI)
- Running builds/tests (use Bash)
- Creating files (use Write tool)

## Authentication Gates

When Claude tries CLI/API and gets auth error, this is NOT a failure - it's a gate requiring human input to unblock automation.

**Pattern:** Claude tries automation → auth error → records observation → continues or defers

**In autonomous mode (Ralph):** Authentication gates should be recorded as observations and the task deferred if blocking.

```xml
<observation type="blocker">
  <title>Vercel CLI requires authentication</title>
  <description>vercel --yes returned "Error: Not authenticated". User needs to run vercel login.</description>
  <file>.vercel/</file>
</observation>
```

## Manual Task Handling in Ralph

**Autonomous Mode (default):**
- Manual tasks are deferred to phase-end manual plan
- Record as observation with type="finding" noting the deferred task
- Continue with auto tasks
- Manual plan runs at phase end with user interaction

**Interactive Mode (manual plans):**
- Manual tasks pause and await user input
- User approves or provides feedback
- Execution continues after input

## Automation Reference

| Service | CLI/API | Auth Gate |
|---------|---------|-----------|
| Vercel | `vercel` | `vercel login` |
| Railway | `railway` | `railway login` |
| Fly | `fly` | `fly auth login` |
| Stripe | `stripe` + API | API key in .env |
| Supabase | `supabase` | `supabase login` |
| GitHub | `gh` | `gh auth login` |
| Node | `npm`/`pnpm` | N/A |
| Xcode | `xcodebuild` | N/A |

**Rule:** If it has CLI/API, Claude does it. Never ask human to perform automatable work.

## Anti-Patterns

### Bad: Asking human to do automatable work
```json
{
  "type": "manual",
  "action": "Deploy to Vercel by visiting vercel.com/new..."
}
```
**Why bad:** Vercel has a CLI. Should be auto task with `vercel --yes`.

### Bad: Too many manual tasks
```json
{"type": "auto", "name": "Create schema"},
{"type": "manual", "name": "Check schema"},
{"type": "auto", "name": "Create API"},
{"type": "manual", "name": "Check API"}
```
**Why bad:** Verification fatigue. Combine into one manual task at end.

### Good: Single verification at end
```json
{"type": "auto", "name": "Create schema"},
{"type": "auto", "name": "Create API"},
{"type": "auto", "name": "Create UI"},
{"type": "manual", "name": "Verify complete auth flow"}
```

## Writing Good Manual Tasks

**DO:**
- Automate everything with CLI/API before manual task
- Be specific: "Visit https://myapp.vercel.app" not "check deployment"
- Number verification steps
- State expected outcomes
- Use descriptive action field (no separate checkpoint types)

**DON'T:**
- Ask human to do work Claude can automate
- Assume knowledge: "Configure the usual settings"
- Mix multiple verifications in one task
- Place manual tasks before automation completes
- Use checkpoint:* task types (use `type: "manual"` instead)

## Invalid Task Types

These task types are **NOT valid** and will cause validation errors:
- `checkpoint:human-verify`
- `checkpoint:human-action`
- `checkpoint:decision`

All these scenarios should use `type: "manual"` with a descriptive action field that explains what the human needs to do.
