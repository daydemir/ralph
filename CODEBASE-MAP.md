# Codebase Map

Fill this out to help Ralph understand your project structure.

## Repositories

<!-- List your repos relative to this ralph/ folder -->
<!-- Ralph runs from here, so paths should be like ../my-app/ -->

- `../my-app/` - Main application
- `../my-backend/` - Backend API (if separate)

## Tech Stack

<!-- Ralph uses this to know how to build and test -->

- **Language:** TypeScript / Python / Go / etc.
- **Framework:** Next.js / Django / etc.
- **Package Manager:** npm / yarn / pnpm / pip / etc.
- **Test Framework:** Jest / pytest / etc.

## Build & Test Commands

<!-- These commands run from the repo root -->

```bash
# Build
cd ../my-app && npm run build

# Test
cd ../my-app && npm test

# Lint (optional)
cd ../my-app && npm run lint
```

## Key Directories

<!-- Help Ralph navigate your codebase -->

### my-app/
- `src/` - Source code
- `src/components/` - UI components
- `src/api/` - API routes or clients
- `src/utils/` - Utility functions
- `tests/` - Test files

### my-backend/ (if applicable)
- `src/` - Source code
- `src/routes/` - API endpoints
- `src/models/` - Data models
- `tests/` - Test files

## Deploy Configuration

<!-- Tell Ralph which repos should auto-deploy after commit -->
<!-- Set auto-deploy to "yes" for repos that deploy immediately -->
<!-- Set to "no" for repos that need manual release (like iOS apps) -->

| Repo | Auto-Deploy | Deploy Command | Notes |
|------|-------------|----------------|-------|
| my-app | no | Manual release | Requires review |
| my-backend | yes | `cd ../my-backend && npm run deploy` | Deploys to prod |

## Notes

<!-- Any special instructions for Ralph -->

- Database migrations: `npm run migrate`
- Environment: Copy `.env.example` to `.env`
- Requires Node 18+
