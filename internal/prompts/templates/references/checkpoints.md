# Checkpoints Reference

Plans execute autonomously. Checkpoints formalize interaction points where human verification or decisions are needed.

**Core principle:** Claude automates everything with CLI/API. Checkpoints are for verification and decisions, not manual work.

## Checkpoint Types

### checkpoint:human-verify (90% of checkpoints)

**When:** Claude completed automated work, human confirms it works correctly.

**Use for:**
- Visual UI checks (layout, styling, responsiveness)
- Interactive flows (click through wizard, test user flows)
- Functional verification (feature works as expected)
- Audio/video playback quality
- Animation smoothness
- Accessibility testing

**Structure in JSON task:**
```json
{
  "type": "checkpoint:human-verify",
  "action": "Verify the responsive dashboard at /dashboard",
  "verify": "Visit http://localhost:3000/dashboard. Desktop: sidebar visible. Mobile: hamburger menu. No layout shifts.",
  "done": "User confirms dashboard displays correctly"
}
```

### checkpoint:decision (9% of checkpoints)

**When:** Human must make choice that affects implementation direction.

**Use for:**
- Technology selection (which auth provider, which database)
- Architecture decisions (monorepo vs separate repos)
- Design choices (color scheme, layout approach)
- Feature prioritization

**Structure in JSON task:**
```json
{
  "type": "checkpoint:decision",
  "action": "Select authentication provider",
  "options": [
    {"id": "supabase", "name": "Supabase Auth", "pros": "Built-in with DB", "cons": "Less customizable"},
    {"id": "clerk", "name": "Clerk", "pros": "Best DX", "cons": "Paid after 10k MAU"}
  ],
  "done": "Authentication provider selected"
}
```

### checkpoint:human-action (1% - Rare)

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
<observation type="auth-gate" severity="high">
  <title>Vercel CLI requires authentication</title>
  <detail>vercel --yes returned "Error: Not authenticated". User needs to run vercel login.</detail>
  <file>.vercel/</file>
  <action>needs-human-verify</action>
</observation>
```

## Checkpoint Handling in Ralph

**Autonomous Mode (default):**
- Manual tasks are deferred to phase-end manual plan
- Record as observation with `type="manual-checkpoint-deferred"`
- Continue with auto tasks
- Manual plan runs at phase end with user interaction

**Interactive Mode (manual plans):**
- Checkpoints pause and await user input
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

### Bad: Asking human to automate
```json
{
  "type": "manual",
  "action": "Deploy to Vercel by visiting vercel.com/new..."
}
```
**Why bad:** Vercel has a CLI. Should be auto task with `vercel --yes`.

### Bad: Too many checkpoints
```json
{"type": "auto", "name": "Create schema"},
{"type": "checkpoint:human-verify", "name": "Check schema"},
{"type": "auto", "name": "Create API"},
{"type": "checkpoint:human-verify", "name": "Check API"}
```
**Why bad:** Verification fatigue. Combine into one checkpoint at end.

### Good: Single verification checkpoint
```json
{"type": "auto", "name": "Create schema"},
{"type": "auto", "name": "Create API"},
{"type": "auto", "name": "Create UI"},
{"type": "checkpoint:human-verify", "name": "Verify complete auth flow"}
```

## Writing Good Checkpoints

**DO:**
- Automate everything with CLI/API before checkpoint
- Be specific: "Visit https://myapp.vercel.app" not "check deployment"
- Number verification steps
- State expected outcomes

**DON'T:**
- Ask human to do work Claude can automate
- Assume knowledge: "Configure the usual settings"
- Mix multiple verifications in one checkpoint
- Place checkpoints before automation completes
