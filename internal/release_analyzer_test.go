package internal

import (
	"context"
	"errors"
	"testing"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/types"
	llmerrors "release-confidence-score/internal/llm/errors"
)

// mockGitProvider implements types.GitProvider for testing
type mockGitProvider struct {
	name           string
	isCompareURL   func(url string) bool
	fetchResult    *types.Comparison
	fetchGuidance  []types.UserGuidance
	fetchDocs      *types.Documentation
	fetchErr       error
	fetchCallCount int
}

func (m *mockGitProvider) IsCompareURL(url string) bool {
	if m.isCompareURL != nil {
		return m.isCompareURL(url)
	}
	return false
}

func (m *mockGitProvider) FetchReleaseData(ctx context.Context, compareURL string) (*types.Comparison, []types.UserGuidance, *types.Documentation, error) {
	m.fetchCallCount++
	return m.fetchResult, m.fetchGuidance, m.fetchDocs, m.fetchErr
}

func (m *mockGitProvider) Name() string {
	return m.name
}

// mockLLMClient implements providers.LLMClient for testing
type mockLLMClient struct {
	responses  []string
	errors     []error
	callCount  int
	callInputs []string
}

func (m *mockLLMClient) Analyze(userPrompt string) (string, error) {
	m.callInputs = append(m.callInputs, userPrompt)
	idx := m.callCount
	m.callCount++

	if idx < len(m.errors) && m.errors[idx] != nil {
		return "", m.errors[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return "", errors.New("no more mock responses")
}

// newTestAnalyzer creates a ReleaseAnalyzer with mock dependencies
func newTestAnalyzer(githubProvider, gitlabProvider types.GitProvider, llmClient *mockLLMClient) *ReleaseAnalyzer {
	return &ReleaseAnalyzer{
		githubProvider: githubProvider,
		gitlabProvider: gitlabProvider,
		llmClient:      llmClient,
		config: &config.Config{
			ScoreThresholds: config.ScoreThresholds{
				AutoDeploy:     80,
				ReviewRequired: 50,
			},
		},
	}
}

// validLLMResponse returns a valid JSON response that report.GenerateReport can parse
func validLLMResponse() string {
	return `{
		"score": 85,
		"analysis": {
			"summary": "Test summary",
			"key_changes": ["change1"],
			"risk_assessment": "low risk",
			"testing_considerations": ["test1"]
		},
		"action_items": {"critical": [], "important": [], "followup": []},
		"code_analysis": {"summary": "Good", "key_findings": [], "risk_factors": []},
		"infrastructure_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"dependency_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"positive_factors": "Well tested",
		"risk_factors": "None"
	}`
}

func TestAnalyzeStandalone_NoURLs(t *testing.T) {
	ra := newTestAnalyzer(nil, nil, nil)

	_, _, err := ra.AnalyzeStandalone([]string{})
	if err == nil {
		t.Fatal("expected error for empty URLs")
	}
	if err.Error() != "no compare URLs provided" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAnalyzeStandalone_UnsupportedURL(t *testing.T) {
	github := &mockGitProvider{
		name:         "GitHub",
		isCompareURL: func(url string) bool { return false },
	}
	gitlab := &mockGitProvider{
		name:         "GitLab",
		isCompareURL: func(url string) bool { return false },
	}

	ra := newTestAnalyzer(github, gitlab, nil)

	_, _, err := ra.AnalyzeStandalone([]string{"https://unsupported.com/compare/a...b"})
	if err == nil {
		t.Fatal("expected error for unsupported URL")
	}
	if err.Error() != "unsupported compare URL: https://unsupported.com/compare/a...b" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAnalyzeStandalone_FetchError(t *testing.T) {
	github := &mockGitProvider{
		name:         "GitHub",
		isCompareURL: func(url string) bool { return true },
		fetchErr:     errors.New("API error"),
	}
	gitlab := &mockGitProvider{
		name:         "GitLab",
		isCompareURL: func(url string) bool { return false },
	}

	ra := newTestAnalyzer(github, gitlab, nil)

	_, _, err := ra.AnalyzeStandalone([]string{"https://github.com/org/repo/compare/a...b"})
	if err == nil {
		t.Fatal("expected error for fetch failure")
	}
	if err.Error() != "failed to fetch data from https://github.com/org/repo/compare/a...b: API error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAnalyzeStandalone_NilComparison(t *testing.T) {
	github := &mockGitProvider{
		name:         "GitHub",
		isCompareURL: func(url string) bool { return true },
		fetchResult:  nil, // nil comparison
	}
	gitlab := &mockGitProvider{
		name:         "GitLab",
		isCompareURL: func(url string) bool { return false },
	}

	ra := newTestAnalyzer(github, gitlab, nil)

	_, _, err := ra.AnalyzeStandalone([]string{"https://github.com/org/repo/compare/a...b"})
	if err == nil {
		t.Fatal("expected error for nil comparison")
	}
	if err.Error() != "no comparison data returned for https://github.com/org/repo/compare/a...b" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAnalyzeStandalone_Success(t *testing.T) {
	github := &mockGitProvider{
		name:         "GitHub",
		isCompareURL: func(url string) bool { return true },
		fetchResult: &types.Comparison{
			RepoURL: "https://github.com/org/repo",
			DiffURL: "https://github.com/org/repo/compare/a...b",
			Commits: []types.Commit{{SHA: "abc123", Message: "test commit"}},
			Files:   []types.FileChange{{Filename: "test.go", Patch: "+line"}},
		},
	}
	gitlab := &mockGitProvider{
		name:         "GitLab",
		isCompareURL: func(url string) bool { return false },
	}
	llm := &mockLLMClient{
		responses: []string{validLLMResponse()},
	}

	ra := newTestAnalyzer(github, gitlab, llm)

	score, report, err := ra.AnalyzeStandalone([]string{"https://github.com/org/repo/compare/a...b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 85 {
		t.Errorf("expected score 85, got %v", score)
	}
	if report == "" {
		t.Error("expected non-empty report")
	}
}

func TestGetReleaseData_EmptyURLs(t *testing.T) {
	ra := newTestAnalyzer(nil, nil, nil)

	comparisons, guidance, docs, err := ra.getReleaseData([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comparisons) != 0 {
		t.Errorf("expected empty comparisons, got %d", len(comparisons))
	}
	if len(guidance) != 0 {
		t.Errorf("expected empty guidance, got %d", len(guidance))
	}
	if len(docs) != 0 {
		t.Errorf("expected empty docs, got %d", len(docs))
	}
}

func TestGetReleaseData_Deduplication(t *testing.T) {
	github := &mockGitProvider{
		name:         "GitHub",
		isCompareURL: func(url string) bool { return true },
		fetchResult: &types.Comparison{
			RepoURL: "https://github.com/org/repo",
		},
	}
	gitlab := &mockGitProvider{
		name:         "GitLab",
		isCompareURL: func(url string) bool { return false },
	}

	ra := newTestAnalyzer(github, gitlab, nil)

	// Same URL three times
	urls := []string{
		"https://github.com/org/repo/compare/a...b",
		"https://github.com/org/repo/compare/a...b",
		"https://github.com/org/repo/compare/a...b",
	}

	_, _, _, err := ra.getReleaseData(urls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only fetch once due to deduplication
	if github.fetchCallCount != 1 {
		t.Errorf("expected 1 fetch call, got %d", github.fetchCallCount)
	}
}

func TestGetReleaseData_MultipleURLs(t *testing.T) {
	github := &mockGitProvider{
		name: "GitHub",
		isCompareURL: func(url string) bool {
			return url == "https://github.com/org/repo/compare/a...b"
		},
		fetchResult: &types.Comparison{RepoURL: "https://github.com/org/repo"},
	}
	gitlab := &mockGitProvider{
		name: "GitLab",
		isCompareURL: func(url string) bool {
			return url == "https://gitlab.com/org/repo/-/compare/a...b"
		},
		fetchResult: &types.Comparison{RepoURL: "https://gitlab.com/org/repo"},
	}

	ra := newTestAnalyzer(github, gitlab, nil)

	urls := []string{
		"https://github.com/org/repo/compare/a...b",
		"https://gitlab.com/org/repo/-/compare/a...b",
	}

	comparisons, _, _, err := ra.getReleaseData(urls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(comparisons) != 2 {
		t.Errorf("expected 2 comparisons, got %d", len(comparisons))
	}

	// Verify both providers were called
	if github.fetchCallCount != 1 {
		t.Errorf("expected 1 GitHub fetch call, got %d", github.fetchCallCount)
	}
	if gitlab.fetchCallCount != 1 {
		t.Errorf("expected 1 GitLab fetch call, got %d", gitlab.fetchCallCount)
	}
}

func TestGetReleaseData_CollectsUserGuidance(t *testing.T) {
	github := &mockGitProvider{
		name:         "GitHub",
		isCompareURL: func(url string) bool { return true },
		fetchResult:  &types.Comparison{RepoURL: "https://github.com/org/repo"},
		fetchGuidance: []types.UserGuidance{
			{Content: "guidance1", Author: "user1"},
			{Content: "guidance2", Author: "user2"},
		},
	}
	gitlab := &mockGitProvider{
		name:         "GitLab",
		isCompareURL: func(url string) bool { return false },
	}

	ra := newTestAnalyzer(github, gitlab, nil)

	_, guidance, _, err := ra.getReleaseData([]string{"https://github.com/org/repo/compare/a...b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(guidance) != 2 {
		t.Errorf("expected 2 guidance entries, got %d", len(guidance))
	}
}

func TestGetReleaseData_CollectsDocumentation(t *testing.T) {
	github := &mockGitProvider{
		name:         "GitHub",
		isCompareURL: func(url string) bool { return true },
		fetchResult:  &types.Comparison{RepoURL: "https://github.com/org/repo"},
		fetchDocs: &types.Documentation{
			MainDocFile:    ".release-confidence-docs.md",
			MainDocContent: "# Docs",
		},
	}
	gitlab := &mockGitProvider{
		name:         "GitLab",
		isCompareURL: func(url string) bool { return false },
	}

	ra := newTestAnalyzer(github, gitlab, nil)

	_, _, docs, err := ra.getReleaseData([]string{"https://github.com/org/repo/compare/a...b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(docs) != 1 {
		t.Errorf("expected 1 documentation entry, got %d", len(docs))
	}
}

func TestAnalyze_SuccessFirstTry(t *testing.T) {
	llm := &mockLLMClient{
		responses: []string{validLLMResponse()},
	}

	ra := newTestAnalyzer(nil, nil, llm)

	score, report, err := ra.analyze(
		[]*types.Comparison{},
		[]types.UserGuidance{},
		[]*types.Documentation{},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 85 {
		t.Errorf("expected score 85, got %v", score)
	}
	if report == "" {
		t.Error("expected non-empty report")
	}
	if llm.callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", llm.callCount)
	}
}

func TestAnalyze_RetriesOnContextWindowError(t *testing.T) {
	contextErr := &llmerrors.ContextWindowError{
		Provider:   "test",
		StatusCode: 400,
	}

	llm := &mockLLMClient{
		// First call fails with context window error, second succeeds
		errors:    []error{contextErr, nil},
		responses: []string{"", validLLMResponse()},
	}

	ra := newTestAnalyzer(nil, nil, llm)

	score, report, err := ra.analyze(
		[]*types.Comparison{},
		[]types.UserGuidance{},
		[]*types.Documentation{},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 85 {
		t.Errorf("expected score 85, got %v", score)
	}
	if report == "" {
		t.Error("expected non-empty report")
	}
	if llm.callCount != 2 {
		t.Errorf("expected 2 LLM calls, got %d", llm.callCount)
	}
}

func TestAnalyze_FailsOnNonContextError(t *testing.T) {
	llm := &mockLLMClient{
		errors: []error{errors.New("API rate limit exceeded")},
	}

	ra := newTestAnalyzer(nil, nil, llm)

	_, _, err := ra.analyze(
		[]*types.Comparison{},
		[]types.UserGuidance{},
		[]*types.Documentation{},
	)

	if err == nil {
		t.Fatal("expected error for non-context error")
	}
	if llm.callCount != 1 {
		t.Errorf("expected 1 LLM call (no retry), got %d", llm.callCount)
	}
}

func TestAnalyze_ExhaustsAllTruncationLevels(t *testing.T) {
	contextErr := &llmerrors.ContextWindowError{
		Provider:   "test",
		StatusCode: 400,
	}

	// All calls fail with context window error
	llm := &mockLLMClient{
		errors: []error{contextErr, contextErr, contextErr, contextErr, contextErr},
	}

	ra := newTestAnalyzer(nil, nil, llm)

	_, _, err := ra.analyze(
		[]*types.Comparison{},
		[]types.UserGuidance{},
		[]*types.Documentation{},
	)

	if err == nil {
		t.Fatal("expected error after exhausting all truncation levels")
	}
	// 1 initial + 4 truncation levels
	if llm.callCount != 5 {
		t.Errorf("expected 5 LLM calls, got %d", llm.callCount)
	}
}

func TestAnalyze_FailsDuringTruncationRetry(t *testing.T) {
	contextErr := &llmerrors.ContextWindowError{
		Provider:   "test",
		StatusCode: 400,
	}

	llm := &mockLLMClient{
		// First fails with context error, second fails with different error
		errors: []error{contextErr, errors.New("network error")},
	}

	ra := newTestAnalyzer(nil, nil, llm)

	_, _, err := ra.analyze(
		[]*types.Comparison{},
		[]types.UserGuidance{},
		[]*types.Documentation{},
	)

	if err == nil {
		t.Fatal("expected error")
	}
	if llm.callCount != 2 {
		t.Errorf("expected 2 LLM calls, got %d", llm.callCount)
	}
}
