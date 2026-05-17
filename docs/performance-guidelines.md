# Performance Guidelines

Conventions and patterns for concurrency, resource management, and API efficiency in this codebase.

## 1. Parallel Fetching with errgroup

All independent I/O operations run in parallel using `golang.org/x/sync/errgroup`. This is the only concurrency primitive used for parallelism in this codebase.

**Rules:**

- Create an errgroup from the incoming `context.Context` so cancellation propagates: `g, gCtx := errgroup.WithContext(ctx)`.
- Use `gCtx` inside goroutines, never the parent `ctx` -- the errgroup context cancels sibling goroutines on first error.
- After `g.Wait()` returns, `gCtx` is canceled. Any sequential work that follows must use the original `ctx`, not `gCtx`. See `user_guidance.go` where comments are processed sequentially after `g.Wait()` using the parent context.
- Collect results into pre-declared variables scoped outside the goroutines, protected by a `sync.Mutex` when multiple goroutines append to the same slice.

**Two-tier parallelism pattern:**

The codebase uses two levels of parallel fetching:

1. **Top level** (`release_analyzer.go`): Multiple compare URLs are fetched in parallel with no concurrency limit -- one goroutine per URL.
2. **Per-URL level** (`FetchReleaseData`): Within each URL, diff+guidance and documentation run as two parallel errgroup goroutines. Diff and guidance are sequential within their goroutine because guidance depends on diff results.

Do not add a third tier of nesting. Keep the fan-out flat.

## 2. Concurrency Limits on API-Heavy Loops

When iterating over a collection and making one API call per item (e.g., enriching each commit with PR/MR metadata), use `g.SetLimit(10)` to cap concurrent API calls.

```go
g, gCtx := errgroup.WithContext(ctx)
g.SetLimit(10)

for i, commit := range allCommits {
    g.Go(func() error {
        comparison.Commits[i] = buildCommitEntry(gCtx, client, commit, owner, repo, cache)
        return nil
    })
}
g.Wait()
```

**Rules:**

- The limit of 10 is the standard in this codebase. Do not change it without measuring rate limit impact.
- Write directly to a pre-allocated slice by index (`Commits[i]`) to avoid needing a mutex. This is safe because each goroutine writes to a distinct index.
- These bounded goroutines always return `nil` -- errors are handled inside `buildCommitEntry` by logging and returning a partial result. This prevents one failed commit from aborting the entire enrichment.

## 3. Thread-Safe Caching with sync.RWMutex

PR and MR objects are cached per-execution to avoid redundant API calls (multiple commits often map to the same PR/MR). Both `prCache` (GitHub) and `mrCache` (GitLab) follow the identical pattern:

**Rules:**

- Use `sync.RWMutex` with read-lock for cache lookups and write-lock for cache inserts. Do not use a plain `sync.Mutex`.
- The `getOrFetchPR`/`getOrFetchMR` methods use the check-release-fetch-store pattern: check cache under `RLock`, release lock, fetch if missing, store under `Lock`. These cache methods use manual `Lock()`/`Unlock()` without `defer` for the short critical sections.
- Cache instances are scoped to a single compare URL execution and passed through the call chain via a `cache` parameter. They are not global.
- When adding a new cache type (e.g., for a new API object), replicate the `prCache`/`mrCache` struct and `getOrFetchPR`/`getOrFetchMR` method pattern exactly. Keep GitHub and GitLab implementations symmetric.

## 4. Mutex for Shared Slice Appends

When multiple errgroup goroutines append to shared result slices, protect appends with a single `sync.Mutex`:

```go
var mu sync.Mutex
var comparisons []*types.Comparison

for _, url := range uniqueURLs {
    g.Go(func() error {
        // ... fetch comparison ...
        mu.Lock()
        defer mu.Unlock()
        comparisons = append(comparisons, comparison)
        return nil
    })
}
```

**Rules:**

- Use one mutex for all shared slices in the same scope (not one mutex per slice).
- Hold the lock only for the append operations, not for the fetch.
- When order matters, use indexed writes to a pre-allocated slice instead (see commit enrichment pattern above).

## 5. HTTP Client Timeouts

HTTP clients are created per-request or per-operation via `httputil.NewHTTPClient()`. There are no shared/global HTTP clients.

**Rules:**

- External URL fetches (documentation): 30-second timeout, hardcoded as `const httpTimeout = 30 * time.Second`.
- LLM API calls: configurable via `RCS_MODEL_TIMEOUT_SECONDS` (default 120 seconds). Applied as `time.Duration(cfg.ModelTimeoutSeconds) * time.Second`.
- Git platform SDK clients (GitHub, GitLab): no explicit timeout set on the HTTP client. Timeouts rely on context cancellation from errgroup.
- Always pass `context.Context` to requests (`http.NewRequestWithContext`, `gitlab.WithContext(ctx)`) so that errgroup cancellation terminates in-flight requests.

## 6. Context Propagation

Every function that makes an API call or I/O operation accepts `context.Context` as its first parameter.

**Rules:**

- `FetchReleaseData`, `fetchDiff`, `fetchUserGuidance`, `buildCommitEntry`, `getOrFetchPR/MR`, and all documentation methods take `ctx`.
- GitLab SDK calls require explicit context passing via `gitlab.WithContext(ctx)`. GitHub SDK methods accept `ctx` as a regular parameter.
- The top-level `getReleaseData` creates its context from `context.Background()` -- there is no request-scoped context above this. Do not change this without adding a timeout or signal handler at the `main` level.

## 7. Pagination

Both GitHub and GitLab APIs require pagination for large result sets.

**Rules:**

- Always use `PerPage: 100` (the maximum) to minimize API round-trips.
- Paginate with a `for` loop checking `resp.NextPage == 0` as the termination condition.
- GitHub comparison API requires special pagination: store the comparison metadata from page 1 only, accumulate commits from all pages.
- Use the generic `fetchAllPaginated[T]` helper (GitHub only) for paginated list endpoints. GitLab pagination is done inline since the SDK API differs.

## 8. Deduplication Before Parallel Work

Deduplicate inputs before spawning goroutines to avoid redundant API calls:

- Compare URLs are deduplicated in `getReleaseData` before parallel fetching.
- PR/MR numbers are tracked with `processedPRs`/`processedMRs` maps in user guidance extraction to avoid processing the same PR/MR twice.

## 9. Progressive Truncation for LLM Retries

When the LLM rejects input due to context window limits, the system retries with progressively more aggressive truncation levels: `low -> moderate -> high -> extreme`.

**Rules:**

- Only retry on `ContextWindowError` (detected by status code 400/413/429 and body keyword matching). All other errors fail immediately.
- Each retry level is a full re-format and re-send -- there is no caching of intermediate formatted prompts.
- Truncation is risk-aware: critical files (auth, DB, API schemas) are never truncated. Low-risk files (docs, tests) are truncated first.
- Documentation linked from the entry point doc is stripped entirely at `high` and `extreme` levels.

## 10. Credential Cleanup

Sensitive credentials are zeroed after use:

```go
clear(cfg.GCPServiceAccountKey)
cfg.GCPServiceAccountKey = nil
```

This happens immediately after `google.CredentialsFromJSONWithTypeAndParams` returns, whether it succeeds or fails.

## 11. Regex Compilation

All regular expressions are compiled at package level (`var fooRegex = regexp.MustCompile(...)`) rather than inside functions. This includes URL matching patterns, documentation section parsing, and bot comment extraction. Never compile a regex inside a loop or a frequently-called function.
