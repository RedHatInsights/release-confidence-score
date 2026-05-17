# Error Handling Guidelines

## Core Rule: Return Errors, Don't Log Them

This codebase follows a strict "return, don't log" policy. Errors propagate up the call stack and are handled at exactly one place: `main.go` calls `log.Fatalf`. Internal packages never call `log.Fatal` or `log.Fatalf`.

The only logging related to errors within internal packages is `slog.Warn` for **non-fatal degradations** (see "Warn-and-Continue" below).

## Error Wrapping Format

All errors use `fmt.Errorf` with `%w` and a short, lowercase, verb-first context prefix:

```go
return fmt.Errorf("failed to fetch comparison: %w", err)
```

**Prefix conventions by layer:**
- API/SDK calls: `"failed to <verb> <noun>: %w"` -- e.g., `"failed to get PR #%d: %w"`
- HTTP operations: short noun prefix -- `"marshal request: %w"`, `"read response: %w"`, `"http request: %w"`
- Parsing: `"invalid <format> format: %s"` with the bad input value appended

Never include the function name in the error message. The verb-noun prefix provides enough context for tracing.

## Config Validation Errors

Config validation uses a distinct pattern: errors name the environment variable and show the invalid value.

```go
return fmt.Errorf("%s must be a valid integer, got: %s", key, str)
return fmt.Errorf("%s must be between %d and %d, got: %d", key, min, max, val)
return fmt.Errorf("RCS_LOG_FORMAT must be one of: %v; got: %s", validLogFormats, cfg.LogFormat)
```

Rules:
- Required-but-missing variables: `"<VAR_NAME> environment variable is required"`
- Invalid values: `"<VAR_NAME> must be <constraint>, got: <value>"`
- Cross-field validation: describe both fields and their values
- `config.Load()` returns `nil, err` on any validation failure; callers never see a partial config

## Custom Error Type: ContextWindowError

The only custom error type is `ContextWindowError` in `internal/llm/errors/`. It represents LLM context window overflow and enables retry logic.

**Structure:**
```go
type ContextWindowError struct {
    StatusCode int
    Message    string
    Provider   string   // "Claude" or "Gemini"
}
```

**Detection pattern (used identically in both LLM providers):**
```go
if llmerrors.IsContextWindowError(resp.StatusCode, body) {
    return "", &llmerrors.ContextWindowError{
        StatusCode: resp.StatusCode,
        Message:    string(body),
        Provider:   "Claude",
    }
}
return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
```

**Consumption via type assertion (not `errors.As`):**
```go
contextErr, ok := err.(*llmerrors.ContextWindowError)
if !ok {
    return 0, "", fmt.Errorf("failed to analyze: %w", err)
}
```

When adding new custom error types, follow this pattern: struct with exported fields, detection helper (`IsXxxError`), and type assertion at the call site.

## Retry Logic

Retry exists only for context window errors, using progressive truncation levels: low -> moderate -> high -> extreme.

Rules:
- On `ContextWindowError`: log a warning, try the next truncation level
- On any other error during retry: fail immediately with wrapping
- After exhausting all levels: wrap the last error with `"failed to analyze even with extreme truncation: %w"`

```go
if _, isContextErr := err.(*llmerrors.ContextWindowError); isContextErr {
    lastErr = err
    continue    // try next truncation level
}
return "", nil, fmt.Errorf("failed to analyze with %s truncation: %w", level, err)
```

There is no general-purpose retry, backoff, or circuit breaker in this codebase.

## Concurrent Error Handling with errgroup

All parallel work uses `golang.org/x/sync/errgroup`. The pattern is consistent across the codebase:

```go
g, gCtx := errgroup.WithContext(ctx)

g.Go(func() error {
    result, err := doWork(gCtx, ...)
    if err != nil {
        return fmt.Errorf("failed to do work: %w", err)
    }
    mu.Lock()
    defer mu.Unlock()
    results = append(results, result)
    return nil
})

if err := g.Wait(); err != nil {
    return nil, err
}
```

Rules:
- Always pass `gCtx` (the errgroup context) to work inside `g.Go`
- Use `g.SetLimit(10)` for API-calling goroutines to avoid rate limiting
- Protect shared slices with `sync.Mutex` when multiple goroutines append
- When goroutines fetch data needed later (e.g., comments after `g.Wait()`), use the original `ctx`, not `gCtx` which is canceled after `Wait()` returns

## Warn-and-Continue Pattern

Some errors are intentionally non-fatal. The codebase uses `slog.Warn` + early return (not error propagation) when:

1. **Enrichment fails for a single item in a batch** -- e.g., failing to find a PR/MR for one commit should not abort the entire comparison:
   ```go
   prNumber, err := getPRForCommit(ctx, client, owner, repo, entry.SHA)
   if err != nil {
       slog.Warn("Failed to find PR for commit", "commit", entry.ShortSHA, "error", err)
       return entry
   }
   ```

2. **Optional data is missing** -- e.g., documentation file not found:
   ```go
   mainDocContent, err := d.source.FetchFileContent(ctx, mainDocFilename, defaultBranch)
   if err != nil {
       slog.Debug("No main documentation file found", "repo", repository.URL, "error", err)
       return &types.Documentation{Repository: repository}, nil
   }
   ```

3. **Additional documentation fetch fails** -- tracked in `failedDocs` map for reporting, but processing continues.

The rule: if the operation is supplementary and the caller can produce a useful result without it, warn and continue. If the operation is required for the caller to function, return the error.

## Sensitive Data in Errors

Authentication token errors intentionally omit the underlying error to avoid leaking credential details:

```go
tok, err := c.tokenSource.Token()
if err != nil {
    return "", fmt.Errorf("failed to obtain authentication token")  // No %w
}
```

GCP credential parsing follows the same rule. After a credential error, the raw key bytes are zeroed and nilled before returning.

## Panic Usage

`panic` is used in exactly one place: the `init()` function in `truncation.go` when an embedded JSON file fails to parse. This is acceptable because:
- The file is embedded at compile time
- A parse failure indicates a programming error
- It will be caught during development, never in production

Do not add new `panic` calls. All runtime errors should be returned.

## errgroup Goroutines That Never Return Errors

In `diff.go` (both GitHub and GitLab), commit augmentation goroutines always return `nil`:

```go
g.Go(func() error {
    comparison.Commits[i] = buildCommitEntry(gCtx, client, commit, owner, repo, cache)
    return nil
})
```

This is intentional. `buildCommitEntry` handles its own errors via warn-and-continue, returning partial data. The errgroup is used purely for concurrency control (`g.SetLimit(10)`), not error propagation.

## GitHub/GitLab Error Message Parity

Error messages for equivalent operations must match across platforms, differing only in platform-specific terminology:

| GitHub | GitLab |
|--------|--------|
| `"failed to get PR #%d: %w"` | `"failed to get MR !%d: %w"` |
| `"failed to find PRs for commit %s: %w"` | `"failed to get MRs for commit %s: %w"` |

When adding error handling to one platform, add the equivalent to the other.

## CLI Validation Errors

CLI validation errors include actionable help text:

```go
return fmt.Errorf("standalone mode requires compare URLs\n\nTry:\n  rcs --compare-links <url1>,<url2>\n\nOr run 'rcs --help' for more information")
```

This pattern applies only to user-facing CLI argument errors, not to internal errors.
