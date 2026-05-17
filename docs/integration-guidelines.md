# Integration Guidelines

## Architecture Overview

This codebase integrates with GitHub, GitLab, GCP Vertex AI (Claude and Gemini), and external URLs. All git platform integrations implement the `GitProvider` interface (`internal/git/types/interfaces.go`), and all LLM integrations implement the `LLMClient` interface (`internal/llm/providers/interface.go`).

## Git Platform Parity (GitHub / GitLab)

The `internal/git/github/` and `internal/git/gitlab/` packages mirror each other structurally. Every file in one package has an equivalent in the other:

| Concern | GitHub file | GitLab file |
|---|---|---|
| SDK client creation | `client.go` | `client.go` |
| Diff fetching + commit augmentation | `diff.go` | `diff.go` |
| User guidance extraction | `user_guidance.go` | `user_guidance.go` |
| Documentation source | `documentation_source.go` | `documentation_source.go` |
| Orchestration | `release_data_fetcher.go` | `release_data_fetcher.go` |

Rules:
- Both platforms use the same `types.Comparison`, `types.UserGuidance`, and `types.Documentation` structs. Never add platform-specific fields to shared types.
- Both use a thread-safe cache (`prCache` / `mrCache`) with `sync.RWMutex` to deduplicate PR/MR API calls across commit augmentation and user guidance. When adding new cached resources, follow this same pattern.
- Both use `errgroup.WithContext` with `g.SetLimit(10)` for parallel commit augmentation. Keep the concurrency limit at 10 to avoid API rate limiting.
- Both use `shared.ExtractQELabel()` and `shared.ParseUserGuidance()` for cross-platform logic. Put platform-agnostic logic in `internal/git/shared/`, not in the platform packages.
- Error message format differs by platform: GitHub uses `"PR #%d"`, GitLab uses `"MR !%d"`. Keep this convention.

## GitHub SDK Conventions

- SDK: `github.com/google/go-github/v86/github`. Import alias: `githubapi` in `release_data_fetcher.go`, bare `github` in other files.
- Client creation: `github.NewClient(nil).WithAuthToken(cfg.GitHubToken)` -- no custom HTTP client needed. Token comes from `RCS_GITHUB_TOKEN`.
- Compare URL regex: `githubCompareRegex` anchored with `$` at end. Extracts exactly 4 groups: owner, repo, base, head.
- Pagination: Use `fetchAllPaginated[T]` generic helper for list endpoints (comments, reviews). Use manual pagination loop for `CompareCommits` (which returns a composite object, not a list).
- Always use `GetXxx()` safe accessors on GitHub SDK objects (e.g., `commit.GetSHA()`, `pr.GetNumber()`). Never dereference pointers directly.
- Authorization check: PR author OR approved reviewer with `AuthorAssociation` in `{OWNER, MEMBER, COLLABORATOR}`.

## GitLab SDK Conventions

- SDK: `gitlab.com/gitlab-org/api/client-go/v2`. Import alias: `gitlabapi` in `release_data_fetcher.go` and `app_interface.go`, bare `gitlab` in other files.
- Client creation requires `gitlab.WithBaseURL(cfg.GitLabBaseURL)`. The base URL (`RCS_GITLAB_BASE_URL`) is mandatory when a GitLab token is provided.
- SSL skip: only the GitLab client supports `SkipSSLVerify`. When enabled, pass a custom `*http.Client` via `gitlab.WithHTTPClient()`.
- Project path must be URL-encoded via `url.PathEscape()` for all API calls. Do this once and pass the encoded path to downstream functions.
- Context passing: use `gitlab.WithContext(ctx)` as the last argument to every SDK call. This is a functional option, not a field on the options struct.
- Compare URL regex: `gitlabCompareRegex` is NOT anchored at end (allows query params). Supports nested groups (`group/subgroup/repo`).
- GitLab diffs don't include per-file stats. `parsePatchStats()` in `diff.go` counts `+`/`-` lines from the unified diff, skipping `+++`/`---` headers.
- Authorization check: MR author OR user in `MergeRequestApprovals.GetConfiguration()` approvers list.

## GCP OAuth2 Authentication

All LLM calls go through GCP Vertex AI. Authentication flow in `release_analyzer.go`:

1. `RCS_GOOGLE_SA_KEY_B64` env var contains base64-encoded GCP service account JSON.
2. Decoded at config load time into `cfg.GCPServiceAccountKey`.
3. `google.CredentialsFromJSONWithTypeAndParams()` creates credentials with scope `https://www.googleapis.com/auth/cloud-platform`.
4. The raw key bytes are zeroed immediately after credential creation: `clear(cfg.GCPServiceAccountKey)` then `cfg.GCPServiceAccountKey = nil`.
5. The `oauth2.TokenSource` is passed to the LLM provider factory and used per-request via `tokenSource.Token()`.

## LLM Provider Conventions

Both providers (`claude.go`, `gemini.go`) follow the same structure:
- Accept `*config.Config` and `oauth2.TokenSource` at construction.
- Build HTTP requests manually (no SDK). Use `httputil.NewHTTPClient()` with timeout from `cfg.ModelTimeoutSeconds` and SSL skip from `cfg.ModelSkipSSLVerify`.
- Set `Authorization: Bearer <token>` header using `tokenSource.Token().AccessToken`.
- Temperature is hardcoded to `0` for deterministic output.
- Check non-200 responses for context window errors via `llmerrors.IsContextWindowError()` before returning generic errors.

Provider-specific differences:
- **Claude**: Endpoint pattern is `{ModelAPI}/anthropic/models/{ModelID}:streamRawPredict`. System prompt is a separate field.
- **Gemini**: Endpoint pattern is `{ModelAPI}/v1beta/openai/chat/completions` (OpenAI-compatible). System prompt is prepended to the user message.

## HTTP Client

All HTTP clients are created through `internal/http/httpclient.go`. Rules:
- Never create `http.Client{}` directly. Always use `httputil.NewHTTPClient(opts)`.
- Only set `SkipSSLVerify: true` when the target is a self-hosted GitLab or when `cfg.ModelSkipSSLVerify` is set. Never skip SSL for GitHub.
- Custom transport is only configured when SSL skip is needed. Otherwise, use the default transport.

## External URL Fetching

`internal/git/shared/external_url_fetcher.go` fetches documentation from external URLs:
- Timeout: 30 seconds (hardcoded, separate from model timeout).
- GitLab URLs get `PRIVATE-TOKEN` header and respect `GitLabSkipSSLVerify`.
- GitLab detection: hostname is `gitlab.com`, `www.gitlab.com`, or starts with `gitlab.`.
- Blob-to-raw URL conversion: replaces `/blob/` with `/raw/` in the path.

## App-Interface Mode (CI/CD Pipeline)

`internal/app_interface/app_interface.go` handles the CI/CD integration:
- Project ID is hardcoded: `"service/app-interface"`.
- Diff URLs come from a comment posted by `devtools-bot` that starts with `"Diffs:"`. URLs are extracted from lines starting with `"- "`.
- User guidance from app-interface MR notes is always marked `IsAuthorized: true` (no authorization check).
- Report posting: `client.Notes.CreateMergeRequestNote()` with the full report as the note body.

## Shared Logic Location

| Logic | Location | Why shared |
|---|---|---|
| QE label extraction | `shared/qe_labels.go` | Same label names on both platforms |
| User guidance parsing (`/rcs` prefix) | `shared/user_guidance_parser.go` | Same format on both platforms |
| Documentation fetching | `shared/documentation_fetcher.go` | Same `.release-confidence-docs.md` entry point |
| External URL fetching | `shared/external_url_fetcher.go` | GitLab auth/SSL logic needed by both platforms |
