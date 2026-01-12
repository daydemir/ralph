# Ralph

Autonomous PRD execution loop for [Claude Code](https://claude.ai/code).

## Installation

### Homebrew (recommended)

```bash
brew tap daydemir/tap
brew install ralph
```

### From source

```bash
go install github.com/daydemir/ralph/cmd/ralph@latest
```

## Quick Start

```bash
# Initialize a workspace
ralph init

# Edit .ralph/codebase-map.md with your project structure

# Start planning mode to create PRDs
ralph plan

# Build a specific PRD
ralph build my-feature

# Run autonomous loop
ralph build --loop
```

## Commands

| Command | Description |
|---------|-------------|
| `ralph init` | Initialize a new workspace |
| `ralph plan` | Interactive planning mode |
| `ralph build [prd-id]` | Build a specific PRD or select interactively |
| `ralph build --loop [N]` | Autonomous loop (default 10 iterations) |
| `ralph list` | List PRDs with status |
| `ralph config` | View/modify configuration |

## Workspace Structure

```
.ralph/
├── config.yaml         # Configuration
├── prd.json            # PRD backlog
├── prd-completed.json  # Completed PRDs archive
├── prompts/            # Customizable prompts
│   ├── plan.md
│   └── build.md
├── codebase-map.md     # Project documentation
├── progress.txt        # Agent memory
└── fix_plan.md         # Known issues
```

## Configuration

Edit `.ralph/config.yaml`:

```yaml
llm:
  backend: claude       # claude | kilocode
  model: sonnet

claude:
  binary: claude
  allowed_tools:
    - Read
    - Write
    - Edit
    - Bash
    - Glob
    - Grep
    - Task
    - TodoWrite

build:
  default_loop_iterations: 10
```

## PRD Format

```json
{
  "features": [
    {
      "id": "my-feature",
      "description": "Add user authentication",
      "steps": [
        "Create auth middleware",
        "Add login endpoint",
        "Write tests"
      ],
      "passes": false
    }
  ]
}
```

## Previous Version

The shell script version is archived in `archive/v0-shell/`.

## License

MIT
