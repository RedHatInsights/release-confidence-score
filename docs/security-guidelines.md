# Security Guidelines

## 1. Authentication Architecture

### GCP OAuth2 for LLM APIs (Primary Auth)
Both LLM providers (Claude, Gemini) authenticate via GCP OAuth2 service account tokens -- not static API keys.

**Rules:**
- All LLM clients receive an `oauth2.TokenSource`, not raw credentials. Tokens are obtained per-request via `tokenSource.Token()`.
- The `Authorization: Bearer` header is set from the `TokenSource` on every request; never cache or store the access token string outside the OAuth2 library.
- When obtaining credentials fails, wipe the key material before returning the error (see credential clearing below).

### Git Platform Tokens (PATs)
GitHub and GitLab use static personal access tokens passed via environment variables.

**Rules:**
- GitHub: Token is passed to `github.NewClient(nil).WithAuthToken(token)`. Never construct auth headers manually for the GitHub SDK.
- GitLab: Token is passed to `gitlab.NewClient(token, ...)`. For raw HTTP requests to GitLab URLs, use the `PRIVATE-TOKEN` header (not `Authorization: Bearer`).

```go
// Correct: GitLab raw HTTP auth
req.Header.Set("PRIVATE-TOKEN", d.config.GitLabToken)
```

## 2. Credential Lifecycle: Clear After Use

The GCP service account key (`GCPServiceAccountKey` in `Config`) is a `[]byte` that must be wiped from memory after credential initialization succeeds or fails.

**Rules:**
- After calling `google.CredentialsFromJSONWithTypeAndParams`, immediately call `clear(cfg.GCPServiceAccountKey)` and set it to `nil`, regardless of success or failure.
- This pattern lives in `internal/release_analyzer.go` `New()`. Any new code path that reads `GCPServiceAccountKey` must follow the same wipe pattern.
- The `Config` struct comment documents this: `GCPServiceAccountKey []byte // cleared after credential initialization`.

```go
creds, err := google.CredentialsFromJSONWithTypeAndParams(context.Background(), cfg.GCPServiceAccountKey, ...)
if err != nil {
    clear(cfg.GCPServiceAccountKey)
    cfg.GCPServiceAccountKey = nil
    return nil, fmt.Errorf("failed to parse GCP service account credentials")
}
clear(cfg.GCPServiceAccountKey)
cfg.GCPServiceAccountKey = nil
```

## 3. Secret Storage and .gitignore

**Rules:**
- `.env` is gitignored; `.env.example` is committed with placeholder values only (e.g., `your_github_token_here`).
- The binary (`/rcs`) and `.claude/` directory are also gitignored.
- Never commit real tokens. Test files use obvious fake values like `"github-token"`, `"gitlab-token"`, `"dGVzdA=="` (base64 of "test").
- `RCS_GOOGLE_SA_KEY_B64` is base64-encoded in the environment to handle JSON special characters. Decode with `base64.StdEncoding.DecodeString` and `strings.TrimSpace` the input first.

## 4. Error Messages: Never Leak Credentials

**Rules:**
- When credential parsing fails, return a generic message without echoing the input. Example: `"RCS_GOOGLE_SA_KEY_B64 contains invalid base64 encoding"` -- not `"failed to decode: <raw value>"`.
- When OAuth2 token acquisition fails, return `"failed to obtain authentication token"` without wrapping the underlying error (which may contain token details).
- For API errors, it is acceptable to include the HTTP status code and response body, since these come from the server, not from local secrets.

## 5. TLS Configuration

**Rules:**
- TLS verification is enabled by default. SSL skip is opt-in via boolean env vars (`RCS_GITLAB_SKIP_SSL_VERIFY`, `RCS_MODEL_SKIP_SSL_VERIFY`), both defaulting to `false`.
- SSL skip is scoped per-subsystem: GitLab client and LLM model client have independent skip flags. Never apply a global skip.
- The `SkipSSLVerify` flag in `HTTPClientOptions` only creates a custom `Transport` when `true`; otherwise the default Go TLS behavior applies.
- For external URL fetching (documentation links), SSL skip is applied only when the URL is detected as a GitLab URL AND `GitLabSkipSSLVerify` is `true`. Non-GitLab external URLs always verify TLS.

```go
skipSSLVerify := isGitLab && d.config.GitLabSkipSSLVerify
```

## 6. HTTP Client Configuration

**Rules:**
- Always set a timeout on HTTP clients. LLM clients use `ModelTimeoutSeconds` (default 120s). External URL fetching uses a 30-second constant (`httpTimeout`).
- Use the shared `httputil.NewHTTPClient()` factory -- never construct `http.Client{}` directly. This ensures consistent TLS and timeout configuration.
- Limit concurrent API calls with `errgroup.SetLimit(10)` to avoid rate-limiting from GitHub/GitLab APIs.

## 7. Input Validation

### Environment Variables
- All env vars are validated at startup in `config.Load()` before any network calls.
- Integer env vars are range-checked (`parseIntEnvOrDefault` enforces `min`/`max`).
- Boolean env vars use `strconv.ParseBool` (accepts `true/false/1/0` only; rejects `yes/no`).
- Log format and level are validated against allowlists.
- If `GitLabToken` is set, `GitLabBaseURL` is required. If mode is `app-interface`, `GitLabToken` is required.

### URL Parsing
- Compare URLs are validated by regex before processing: `githubCompareRegex` and `gitlabCompareRegex`. Invalid URLs are rejected with an explicit error.
- GitLab project paths are URL-encoded with `url.PathEscape` before use in API calls.
- GitLab URL detection for SSL config uses `url.Parse` with a fallback to string matching if parsing fails.

## 8. User Guidance Authorization

User guidance (comments that influence the LLM analysis) is filtered by authorization status before being included in the prompt.

**Rules:**
- Only authorized guidance reaches the LLM. The function `extractAuthorizedGuidance` filters on `IsAuthorized == true`.
- GitHub authorization: user must be the PR author OR have an `APPROVED` review with `AuthorAssociation` of `OWNER`, `MEMBER`, or `COLLABORATOR`. Random commenters cannot influence scoring.
- GitLab authorization: user must be the MR author OR be in the MR's approvers list. GitLab's approval system is permission-based, so approver presence is sufficient.
- App-interface (GitLab) guidance is always marked `IsAuthorized: true` because it comes from a controlled internal MR.

## 9. Container Security

**Rules:**
- Multi-stage Docker build: build stage uses `ubi9/go-toolset`, runtime uses `ubi9/ubi-minimal`. Build tools are not present in the final image.
- Runtime container runs as non-root user (`USER 1001`).
- Binary is built with `CGO_ENABLED=0` (static linking, no C dependencies in the image).
- Tekton pipelines use `hermetic: 'true'` for reproducible, network-isolated builds.
- Git auth in Tekton uses a Kubernetes secret reference (`git_auth_secret`), not inline credentials.

## 10. CI/CD Security

**Rules:**
- GitHub Actions workflow sets `permissions: contents: read` (principle of least privilege).
- GitHub Actions are pinned to full commit SHAs, not tags, to prevent supply chain attacks via tag mutation.
- Tekton pipeline references use versioned URLs (e.g., `v1.67.1`), not `latest` or `main`.
- Tekton service accounts are scoped per-component (`build-pipeline-rcs-app-interface`).

## 11. Logging Discipline

**Rules:**
- Credentials and tokens are never logged. The `slog.Debug` calls in LLM clients log request/response payloads (which contain prompts and analysis, not auth headers) but the `Authorization` header is set after the debug log.
- Rate limit remaining counts are logged at debug level (safe metadata).
- Debug logging is off by default (log level defaults to `info`). Enable with `RCS_LOG_LEVEL=debug` only in non-production.

## 12. Security-Sensitive File Detection (LLM Prompt)

The system prompt instructs the LLM to flag security concerns in analyzed diffs. When modifying system prompts:
- Maintain the "Security & Sensitive Data" checklist: hardcoded secrets, logging in auth/PII paths, error message leakage, input validation changes, dependency CVEs.
- File criticality tiers must classify `auth/`, security changes, and authentication as "Critical".
