# AGENTS.md

Agent onboarding guide for the Release Confidence Score (RCS) codebase. This document covers cross-cutting conventions, architectural context, and pointers to domain-specific guidelines. It applies to all AI agents (Claude, Cursor, CodeRabbit, etc.).

For project overview, operation modes, configuration, and environment variables, see [README.md](README.md).

## Guideline Index

Detailed domain-specific rules are maintained in separate guideline files. Read the relevant guidelines before modifying any code in their scope.

| Guideline file | Domain |
|---|---|
| [docs/security-guidelines.md](docs/security-guidelines.md) | Authentication, credential handling, TLS, input validation |
| [docs/performance-guidelines.md](docs/performance-guidelines.md) | Concurrency, caching, rate limiting, context propagation |
| [docs/error-handling-guidelines.md](docs/error-handling-guidelines.md) | Error wrapping, custom types, retry logic, warn-and-continue |
| [docs/api-contracts-guidelines.md](docs/api-contracts-guidelines.md) | GitProvider interface, SDK patterns, pagination, LLM clients |
| [docs/testing-guidelines.md](docs/testing-guidelines.md) | Table-driven tests, mocking, assertion style, test parity |
| [docs/integration-guidelines.md](docs/integration-guidelines.md) | GitHub/GitLab/GCP integration, shared logic, app-interface |

## Code Quality Principles

- Write the simplest code that solves the problem. Avoid abstractions until clearly needed (rule of three).
- Remove dead code immediately. Check if a function/variable is actually used before keeping it.
- Do not store data that can be computed or already exists. Do not rebuild values when you have them (e.g., do not reconstruct URLs from parts when you have the original URL).
- If data flows IN somewhere, do not create a method to pull it back OUT.
- Trust internal code; only validate at system boundaries. Do not add error handling for impossible scenarios.

## GitHub/GitLab Parity

The `internal/git/github/` and `internal/git/gitlab/` packages implement the same `GitProvider` interface. When modifying one:

- Check if the same change applies to the other platform.
- Keep function signatures, error messages, and behavior consistent.
- If a fix applies to one platform, it likely applies to both (e.g., URL regex fixes).
- Both packages have mirrored file structures: `client.go`, `diff.go`, `user_guidance.go`, `documentation_source.go`, `release_data_fetcher.go`.
- Never add platform-specific fields to shared types in `internal/git/types/`.
- Platform-agnostic logic belongs in `internal/git/shared/`, not in the platform packages.
- When in doubt about whether a change should be mirrored, ask the user before proceeding.

## Architecture

### High-Level Data Flow

```
main.go
  -> cli.Parse()           -- parse flags, determine mode
  -> config.Load()         -- load + validate env vars
  -> internal.New()        -- create ReleaseAnalyzer (GitHub client, GitLab client, GCP OAuth2, LLM client)
  -> AnalyzeAppInterface() or AnalyzeStandalone()
       -> getReleaseData() -- parallel fetch from GitHub/GitLab via GitProvider interface
       -> analyze()        -- format data, call LLM, retry with truncation if needed, render report
```

### Package Layout

```
main.go                           -- Entry point; CLI dispatch, fatal error handling
internal/
  release_analyzer.go             -- Orchestrator: wires providers, runs analysis, handles LLM retry
  cli/                            -- CLI flag parsing and validation
  config/                         -- Environment variable loading and validation
  logger/                         -- slog setup (text/JSON, log level)
  git/
    types/                        -- Platform-agnostic interfaces (GitProvider, DocumentationSource) and data types
    github/                       -- GitHub GitProvider implementation
    gitlab/                       -- GitLab GitProvider implementation (mirrors github/ structure)
    shared/                       -- Cross-platform logic: documentation fetching, QE labels, user guidance parsing, external URL fetching
  llm/
    providers/                    -- LLM client implementations (Claude, Gemini) + factory
    errors/                       -- ContextWindowError custom type
    formatting/                   -- Format comparisons and documentation for the LLM prompt
    prompts/system/               -- Embedded system prompt markdown files + version selector
    prompts/user/                 -- User prompt template rendering
    truncation/                   -- Progressive diff truncation with risk-based file prioritization
  http/                           -- HTTP client factory (timeout, TLS skip)
  report/                         -- Report template rendering
  app_interface/                  -- App-interface mode: GitLab MR note parsing, diff URL extraction, report posting
```

### Key Interfaces

There are two core interfaces that define the extension points:

- **`GitProvider`** (`internal/git/types/interfaces.go`): Three methods -- `IsCompareURL`, `FetchReleaseData`, `Name`. Implemented by `github.Fetcher` and `gitlab.Fetcher`.
- **`LLMClient`** (`internal/llm/providers/interface.go`): Single method -- `Analyze(userPrompt string) (string, error)`. Implemented by `claude` and `gemini` providers.

A third interface, **`DocumentationSource`**, decouples documentation fetching from platform SDKs with `GetDefaultBranch` and `FetchFileContent`.

### Embedded Files

Several files are embedded at compile time via `//go:embed`:

- `internal/llm/prompts/system/system_prompt_v1.md` and `system_prompt_v2.md` -- LLM system prompts
- `internal/llm/prompts/user/user_prompt_template_v1.md` -- User prompt Go template
- `internal/report/report_template.md` -- Report output Go template
- `internal/llm/truncation/truncation.go` embeds a JSON file for file risk classification

Modifying these embedded files changes LLM behavior or report output. Test changes carefully.

### Two Operation Modes

- **App-interface mode**: Reads a GitLab MR from the `service/app-interface` project, extracts diff URLs from a `devtools-bot` comment, fetches release data from those URLs, and optionally posts the report back to the MR.
- **Standalone mode**: Takes compare URLs directly via CLI flags and prints the report to stdout.

Both modes converge at `analyze()` in `release_analyzer.go`.

### Concurrency Model

The codebase uses a two-tier parallelism pattern with `errgroup`:

1. **Top level**: Multiple compare URLs are fetched in parallel (one goroutine per URL, no concurrency limit).
2. **Per-URL level**: Within each URL, diff+guidance and documentation run as two parallel errgroup goroutines. Diff and guidance are sequential within their goroutine because guidance depends on diff results.

API-heavy loops (commit enrichment) use `g.SetLimit(10)` to cap concurrent API calls. Do not add a third tier of nesting.

### Configuration

All configuration comes from `RCS_`-prefixed environment variables, loaded and validated in `config.Load()` at startup. There are no config files, no YAML, no flags for configuration values. CLI flags control only the operation mode and input parameters.

### Build and CI

- **Build**: `go build -o rcs .` or `docker compose build`. Static binary with `CGO_ENABLED=0`.
- **Tests**: `go test ./...`. No build tags, test flags, or special setup required.
- **GitHub Actions** (`.github/workflows/go.yml`): Runs build and test on push/PR to main. Actions are pinned to commit SHAs.
- **Tekton** (`.tekton/`): Konflux/RHTAP pipelines for container image builds. Hermetic builds, scoped service accounts, versioned pipeline references.
- **Renovate** (`renovate.json`): Automated dependency updates via Konflux mintmaker config. Tekton updates auto-merge.

### Dependencies

The codebase has minimal direct dependencies (see `go.mod`):

- `github.com/google/go-github/v86` -- GitHub API SDK
- `gitlab.com/gitlab-org/api/client-go/v2` -- GitLab API SDK
- `golang.org/x/oauth2` -- GCP OAuth2 token source
- `golang.org/x/sync` -- errgroup for structured concurrency

No web frameworks, no assertion libraries, no code generation tools, no mocking libraries.

### Friction Capture and Continuous Improvement

This repo has an automatic friction capture system. At the end of every Claude Code session, a Stop hook analyzes the conversation transcript and writes friction events (corrections, clarifications, denials, mistakes) as individual markdown files to `.claude/friction/`.

#### For contributors

1. Run `git config core.hooksPath .githooks` once after cloning.
2. Work normally. Friction is captured automatically.
3. Before pushing, if friction files exist, run `/improve-docs`.
4. The command proposes doc and eval improvements, opens a PR, and deletes the friction files.
5. Push goes through. Team reviews the PR normally.

#### Friction file format

Each file in `.claude/friction/` has YAML frontmatter with `type`, `severity`, `slug`, `doc_gap`, `session`, and `date` fields, followed by a one-paragraph description.

#### Evals

Run evals locally with `promptfoo eval` after doc changes. CI runs evals automatically on PRs that touch docs or agent context files (`.github/workflows/evals.yml`).

