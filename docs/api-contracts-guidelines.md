# API Contracts Guidelines

## GitProvider Interface

The `GitProvider` interface (`internal/git/types/interfaces.go`) is the central abstraction for all git platform interactions. Every platform must implement exactly three methods:

- `IsCompareURL(url string) bool` -- URL detection via compiled regex (no network calls)
- `FetchReleaseData(ctx context.Context, compareURL string) (*Comparison, []UserGuidance, *Documentation, error)` -- single entry point that returns all data
- `Name() string` -- returns platform display name ("GitHub" or "GitLab")

Rules:
1. `FetchReleaseData` must return all three data types in one call. Callers never call sub-fetchers directly.
2. The caller dispatches by iterating providers and calling `IsCompareURL` -- never by parsing the URL itself.
3. New providers must follow the same `Fetcher` struct pattern: hold an SDK client and `*config.Config`.

## Compare URL Regex Patterns

GitHub and GitLab use different regex patterns. Both must:
- Anchor refs with `(.+?)\.\.\.([^?#]+)` to handle tags, branches, and SHAs
- Strip query strings and fragments from the head ref via `[^?#]+`

Key differences:
- GitHub regex captures `owner` and `repo` separately; GitLab captures `host` and full `projectPath` (supports nested groups like `group/subgroup/repo`)
- GitHub regex is end-anchored (`$`); GitLab is not (allows trailing path segments)
- GitLab includes `/-/` in the path; GitHub does not
- Both regexes are compiled at package level (`var ... = regexp.MustCompile(...)`)

When fixing a regex issue in one provider, check whether the same fix applies to the other.

## SDK Client Construction

GitHub and GitLab SDK clients are created differently:

- **GitHub**: `github.NewClient(nil).WithAuthToken(token)` -- no error return, no base URL needed
- **GitLab**: `gitlab.NewClient(token, gitlab.WithBaseURL(...))` -- returns `(*Client, error)`, requires base URL, optionally accepts custom `http.Client` for SSL skip

Both are constructed in a `client.go` file within their package. The HTTP client for SSL override comes from `internal/http/httpclient.go`, never built inline.

## LLM Provider Pattern

LLM providers implement `LLMClient` with a single method: `Analyze(userPrompt string) (string, error)`.

Conventions every provider must follow:
1. Build endpoint URL from `cfg.ModelAPI` + provider-specific path suffix
2. Create `http.Client` via `httputil.NewHTTPClient` with timeout from `cfg.ModelTimeoutSeconds`
3. Authenticate with GCP OAuth2: call `tokenSource.Token()`, set `Authorization: Bearer` header
4. On non-200 response, check `llmerrors.IsContextWindowError` first; if true, return `*llmerrors.ContextWindowError`. Otherwise return `fmt.Errorf("API error %d: %s", ...)`
5. Log token usage at Debug level after successful response
6. Use `Temperature: 0` for deterministic output
7. Request/response types are provider-specific structs defined in the same file (not shared)

Provider differences:
- Claude sends system prompt as a separate `system` field; Gemini concatenates system+user into a single message
- Claude response is `Content[0].Text`; Gemini response is `Choices[0].Message.Content`

## HTTP Client Factory

All HTTP clients must be created via `httputil.NewHTTPClient(HTTPClientOptions{...})` from `internal/http/httpclient.go`. Never construct `http.Client{}` directly. The factory handles:
- Timeout configuration
- TLS skip (only configures custom transport when `SkipSSLVerify` is true)

## Pagination

### GitHub
Use `github.ListOptions{Page: 1, PerPage: 100}` and loop until `resp.NextPage == 0`:

```go
for {
    items, resp, err := fetcher(ctx, opts)
    // ...
    if resp.NextPage == 0 { break }
    opts.Page = resp.NextPage
}
```

The generic helper `fetchAllPaginated[T]` in `github/user_guidance.go` encapsulates this. Use it for any paginated GitHub endpoint. For the comparison endpoint specifically, pagination is handled in `fetchComparisonWithPagination` because it needs to preserve the first page's metadata.

### GitLab
Same loop structure but with `gitlab.ListOptions{PerPage: 100, Page: 1}`. GitLab has no generic helper -- pagination is inlined. When adding new paginated GitLab calls, follow the existing inline pattern in `user_guidance.go` and `app_interface.go`.

## Concurrency and Rate Limiting

1. Use `errgroup.WithContext(ctx)` for all parallel API work
2. Set `g.SetLimit(10)` when making per-commit API calls to avoid rate limiting
3. Use a thread-safe cache (`sync.RWMutex` + map) to deduplicate PR/MR fetches. GitHub uses `prCache`; GitLab uses `mrCache`. Both follow the same `getOrFetchX` pattern: read-lock check, then write-lock store
4. Log `rate_limit_remaining` from GitHub responses at Debug level
5. Independent data fetches (e.g., documentation vs. diff+guidance) run in parallel goroutines via errgroup. Sequential dependencies (guidance depends on diff) run within the same goroutine

## Platform-Agnostic Types

All API responses must be converted to types in `internal/git/types/types.go` before leaving the platform package:
- GitHub `CommitFile` -> `types.FileChange` via `convertFile()`
- GitLab `Diff` -> `types.FileChange` via `convertDiff()`
- Both use status strings: `"added"`, `"modified"`, `"removed"`, `"renamed"`

GitLab does not provide per-file addition/deletion counts, so `parsePatchStats()` calculates them from the unified diff. GitHub provides these directly via the SDK.

## DocumentationSource Interface

The `DocumentationSource` interface decouples documentation fetching from platform SDKs:
- `GetDefaultBranch(ctx) (string, error)` -- falls back to `"main"` if the repo has no default branch
- `FetchFileContent(ctx, path, ref) (string, error)` -- GitHub SDK auto-decodes base64; GitLab requires manual `base64.StdEncoding.DecodeString` when `file.Encoding == "base64"`

## External URL Fetching

When fetching raw content from external URLs (`shared/external_url_fetcher.go`):
1. Detect GitLab URLs via hostname parsing (not regex) to decide SSL and auth behavior
2. Add `PRIVATE-TOKEN` header for GitLab URLs (not `Authorization: Bearer`)
3. Use a fixed 30-second timeout for external fetches, separate from LLM timeout

Blob-to-raw URL conversion (`shared/documentation_fetcher.go`):
- Convert blob URLs to raw URLs via `strings.Replace(path, "/blob/", "/raw/", 1)` in `fetchAdditionalDocContent` -- works for both GitHub and GitLab URL formats

## Authorization Checks

GitHub and GitLab use different authorization models for user guidance:
- **GitHub**: PR author OR user with `APPROVED` review AND `AuthorAssociation` in `{OWNER, MEMBER, COLLABORATOR}`. Requires fetching reviews via API
- **GitLab**: MR author OR user in MR approvers list (fetched via `MergeRequestApprovals.GetConfiguration`). No association check needed because GitLab's approval system is permission-gated
- **App-interface**: All guidance is `IsAuthorized: true` (trusted context)

## Configuration and Authentication

1. All config comes from environment variables prefixed with `RCS_`. Model-specific vars use the provider name: `RCS_CLAUDE_MODEL_API`, `RCS_GEMINI_MODEL_API`
2. GCP service account key is base64-encoded in `RCS_GOOGLE_SA_KEY_B64`, decoded once, used to create `oauth2.TokenSource`, then cleared from memory
3. GitHub auth: token passed to SDK via `WithAuthToken()`
4. GitLab auth: token passed to SDK constructor. For raw HTTP calls, use `PRIVATE-TOKEN` header
5. GitLab requires `RCS_GITLAB_BASE_URL` when `RCS_GITLAB_TOKEN` is set
