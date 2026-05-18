# Claude Code Instructions

@AGENTS.md

## Build and Test Commands

### Build
```bash
go build -v ./...
```

Or build the binary directly:
```bash
go build -o rcs .
```

### Test
```bash
go test -v ./...
```

### Docker Build
```bash
docker compose build
```

### Run Locally
```bash
# Copy and configure environment
cp .env.example .env
# Edit .env with credentials

# Run with Docker
docker compose run --rm rcs --compare-links "https://github.com/org/repo/compare/v1.0...v1.1"

# Or run the binary directly (after setting environment variables)
./rcs --compare-links "https://github.com/org/repo/compare/v1.0...v1.1"
```

### Environment Requirements
- Go 1.25.0 or later
- No linter configuration (follow Go standard formatting)
- No vendoring (dependencies via `go mod download`)

## Setup (once per clone)

```bash
git config core.hooksPath .githooks
```

To enable friction capture, add to `.claude/settings.local.json`:

```json
{
  "env": {
    "FRICTION_CAPTURE": "1"
  }
}
```
