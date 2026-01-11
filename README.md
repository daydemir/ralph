# Ralph

Autonomous PRD execution loop for [Claude Code](https://claude.ai/code).

## What is Ralph?

Ralph is a system for autonomous code development using Claude Code. The workflow:

1. **You** talk to Claude (as a Product Manager) to define PRDs
2. PRDs get added to `prd.json`
3. `ralph.sh` runs Claude Code in a loop, implementing PRDs one by one
4. You can add new PRDs while Ralph works

Ralph picks a PRD, writes tests, implements the feature, verifies it works, marks it complete, and moves to the next one.

## Prerequisites

- [Claude Code CLI](https://claude.ai/code) installed and authenticated
- `jq` for JSON processing: `brew install jq`

## Setup

1. **Use this template** to create your own repo (click "Use this template" on GitHub)

2. **Clone next to your project repos:**
   ```
   ~/projects/
   ├── ralph/          # This repo (your PRD workspace)
   ├── my-app/         # Your application repo
   └── my-backend/     # Your backend repo (if any)
   ```

3. **Fill in `CODEBASE-MAP.md`** with your repo structure and tech stack

4. **Customize `PROMPT.md`** for your stack (optional - defaults work for most projects)

5. **Add your first PRD** to `prd.json`

6. **Run Ralph:**
   ```bash
   ./ralph.sh        # Run until all PRDs complete (max 10 iterations)
   ./ralph.sh 5      # Run max 5 iterations
   ```

## Adding PRDs

Edit `prd.json` to add features:

```json
{
  "features": [
    {
      "id": "user-authentication",
      "description": "Add login/logout with JWT tokens",
      "steps": [
        "Create auth middleware",
        "Add login endpoint",
        "Add logout endpoint",
        "Write tests for auth flow",
        "Update API documentation"
      ],
      "passes": false
    }
  ]
}
```

Ralph will:
- Pick a PRD where `passes: false`
- Follow the steps
- Set `passes: true` when complete
- Move to the next PRD

## PM Mode

You can talk to Claude to help create PRDs:

```
You: I need to add user authentication to my app

Claude: Let me help you break that down into a PRD...
[Creates structured PRD with steps]

You: Looks good, add it to prd.json

Claude: [Adds PRD to prd.json]
```

Then run `./ralph.sh` to have Ralph implement it.

## File Structure

```
ralph/
├── ralph.sh           # The loop script
├── PROMPT.md          # Instructions for Ralph (customize for your stack)
├── prd.json           # Your PRDs (add features here)
├── progress.txt       # Ralph's memory of completed work
├── fix_plan.md        # Known bugs and issues to address
├── CODEBASE-MAP.md    # Your repo structure and tech stack
└── specs/             # Detailed feature specifications (optional)
```

## Customization

### PROMPT.md

Customize the verification commands for your stack:

```markdown
## 4. Run Verification
- Build: `npm run build`
- Test: `npm test`
- Lint: `npm run lint`
```

### Code Standards

The default PROMPT.md includes code standards (type safety, functional programming, minimal code). Modify these to match your team's practices.

## Sources & Inspiration

- [Original Ralph concept by Geoffrey Huntley](https://ghuntley.com/ralph/)
- [Ralph demo video](https://youtu.be/_IK18goX4X8?si=LSf_Mgjr9ym8pcY8)
- [Anthropic: Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)

## License

MIT
