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

## Claude Code Behavior

### Path-Specific Rules
The `.claude/rules/` directory contains path-based rule imports that automatically load relevant guidelines:
- `github-integration.md` - Loads integration, api-contracts, testing guidelines for `internal/git/github/**`
- `gitlab-integration.md` - Loads integration, api-contracts, testing guidelines for `internal/git/gitlab/**`
- `shared-git.md` - Loads integration, api-contracts guidelines for `internal/git/shared/**`
- `llm.md` - Loads security, api-contracts, error-handling, performance guidelines for `internal/llm/**`
- `config.md` - Loads security, error-handling guidelines for `internal/config/**`
- `app-interface.md` - Loads integration, security guidelines for `internal/app_interface/**`
- `ci-cd.md` - Loads security guidelines for `.github/**` and `.tekton/**`

These rules are automatically applied when you work in those directories.

### Environment Requirements
- Go 1.25.0 or later
- No pre-commit hooks configured
- No linter configuration (follow Go standard formatting)
- No vendoring (dependencies via `go mod download`)
