# Testing Guidelines

## File Organization

- Test files live alongside source files in the same package (e.g., `diff.go` and `diff_test.go`).
- Tests use the same package name as the source (no `_test` suffix on the package declaration), giving access to unexported functions.
- No separate `testdata/` directories, fixtures, or golden files exist in this repo. All test data is constructed inline.

## Table-Driven Tests

Table-driven tests are the dominant pattern. Follow this structure exactly:

- Name the slice `tests` (not `testCases`, `cases`, etc.).
- Use the loop variable `tt` (not `tc` or `test`).
- Every test case must have a `name string` field as the first field.
- Call `t.Run(tt.name, ...)` for every case.

```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"descriptive name", "input", "expected"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // ...
    })
}
```

When a function returns multiple values, use separate `expected`/`want` fields per output (e.g., `wantOwner`, `wantRepo`, `wantErr`). Do not pack multiple outputs into a single struct.

## Subtests Without Tables

For tests that need unique setup per case or test stateful/multi-step behavior, use named subtests directly:

```go
t.Run("single page", func(t *testing.T) { ... })
t.Run("multiple pages", func(t *testing.T) { ... })
```

This pattern is used for pagination tests, combined metadata tests, and truncation scenarios.

## Assertion Style

- Use `t.Errorf` for non-fatal failures and `t.Fatalf` only when subsequent assertions depend on the failed value.
- Use `t.Fatal` / `t.Fatalf` for nil checks that would cause panics downstream.
- No assertion libraries (testify, etc.) are used. Stick to standard `testing` package.
- Format error messages as: `t.Errorf("FunctionName() = %v, want %v", got, want)` or `t.Errorf("field = %q, want %q", got, want)`. Use `%q` for strings, `%v` or `%d` for other types.
- For error cases, check `err == nil` then return early. For expected errors containing a substring, use `strings.Contains(err.Error(), "substring")`.

## Parallel Tests

`t.Parallel()` is **not used** anywhere in this codebase. Do not add it.

## Mocking External Services

### httptest for HTTP APIs

LLM provider tests and external URL fetch tests use `httptest.NewServer` to mock HTTP endpoints. Always `defer server.Close()`. Pass `server.URL` as the API endpoint via config:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Verify request properties
    // Write response
}))
defer server.Close()

cfg := &config.Config{ModelAPI: server.URL, ...}
```

Provider tests verify request method, Content-Type, and Authorization headers inside the handler.

### Interface Mocks

Mocks are hand-written structs in test files (no code generation or mocking libraries). They are defined in the test file that uses them, not in shared test helpers.

Three mock patterns exist:

1. **`mockDocumentationSource`** (in `documentation_fetcher_test.go`): Uses maps for file content and errors. Implements `types.DocumentationSource`.

2. **`mockGitProvider`** (in `release_analyzer_test.go`): Uses function fields for flexible behavior per test. Tracks call count.

3. **`mockLLMClient`** (in `release_analyzer_test.go`): Uses response/error slices indexed by call count to simulate multi-call sequences (e.g., retry scenarios).

When a mock needs dynamic behavior per test case, use function fields:
```go
type mockGitProvider struct {
    isCompareURL func(url string) bool
    fetchResult  *types.Comparison
    fetchErr     error
}
```

### OAuth2 Token Mocks

LLM provider tests mock `oauth2.TokenSource` with a `staticTokenSource` for success and an `errorTokenSource` for authentication failures. These are defined once in `claude_test.go` and reused by `gemini_test.go` and `factory_test.go` within the same package.

## Environment Variables in Tests

Use `t.Setenv()` for environment variables. This automatically cleans up after the test. Never use raw `os.Setenv` in new tests.

For CLI tests that modify `os.Args`, save and restore manually:
```go
oldArgs := os.Args
defer func() { os.Args = oldArgs }()
os.Args = tt.args
```

## Testing for Contains vs Equality

- Use exact equality (`!=`, `==`) when testing a single deterministic output.
- Use `strings.Contains` when testing that rendered/formatted output includes expected fragments. This is the standard pattern for template rendering and report generation tests.
- When checking multiple fragments in output, use `expectInOutput` / `expectNotInOutput` string slices in the table struct.

## Nil and Edge Case Coverage

Every function that accepts pointers or slices must be tested with nil inputs. The established patterns:
- Nil pointer input (e.g., `nil PR`, `nil MR`, `nil comparison`)
- Empty slice/map input
- Slice containing nil elements (e.g., `[]*types.Comparison{nil}`)
- Zero-value structs

## GitHub/GitLab Test Parity

When tests exist for `git/github/`, equivalent tests should exist for `git/gitlab/` testing the same logical behaviors. The test case names and structure should mirror each other.

## Skipping Tests That Need Complex Mocks

When a test would require mocking a full SDK client and that mock does not yet exist, use `t.Skip` with a clear reason:

```go
t.Skip("Skipping - requires mock GitLab client to test deduplication logic")
```

Do not write empty or misleading tests. Either test it properly or skip with rationale.

## Test Helper Functions

- Helper functions are defined in the test file that uses them (e.g., `validLLMResponse()` in `release_analyzer_test.go`).
- No shared test utility packages exist. If a mock is needed across files in the same package, it can be defined once and reused (like `mockTS()` in the providers package).
- Factory helpers like `newTestAnalyzer()` encapsulate construction of the system under test with mock dependencies.

## What Is Tested

- **Pure functions**: Converters, parsers, formatters, validators (most tests)
- **HTTP client interactions**: LLM providers via httptest (Claude, Gemini)
- **Configuration loading**: Environment variable parsing and validation
- **CLI argument parsing**: Flag parsing and validation
- **Template rendering**: Report generation with various input combinations
- **Retry/truncation logic**: Context window error handling with multi-call mock sequences

## What Is Not Tested

- Direct GitHub/GitLab SDK calls (tests cover the wrapper logic, not API calls)
- `main.go` and the top-level wiring
- Actual LLM responses or end-to-end flows

## Running Tests

```bash
go test ./...
```

No build tags, test flags, or special setup required.
