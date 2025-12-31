package rest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"release-confidence-score/internal/git/types"

	"github.com/google/go-github/v80/github"
)

func TestIsValidIssueComment(t *testing.T) {
	body := "test body"
	login := "testuser"
	htmlURL := "https://github.com/test"

	tests := []struct {
		name     string
		comment  *github.IssueComment
		expected bool
	}{
		{
			name: "valid comment",
			comment: &github.IssueComment{
				Body:    &body,
				User:    &github.User{Login: &login},
				HTMLURL: &htmlURL,
			},
			expected: true,
		},
		{
			name:     "nil comment",
			comment:  nil,
			expected: false,
		},
		{
			name: "empty body",
			comment: &github.IssueComment{
				Body:    github.String(""),
				User:    &github.User{Login: &login},
				HTMLURL: &htmlURL,
			},
			expected: false,
		},
		{
			name: "nil user",
			comment: &github.IssueComment{
				Body:    &body,
				User:    nil,
				HTMLURL: &htmlURL,
			},
			expected: false,
		},
		{
			name: "empty login",
			comment: &github.IssueComment{
				Body:    &body,
				User:    &github.User{Login: github.String("")},
				HTMLURL: &htmlURL,
			},
			expected: false,
		},
		{
			name: "empty HTML URL",
			comment: &github.IssueComment{
				Body:    &body,
				User:    &github.User{Login: &login},
				HTMLURL: github.String(""),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidIssueComment(tt.comment)
			if result != tt.expected {
				t.Errorf("isValidIssueComment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidReviewComment(t *testing.T) {
	body := "test body"
	login := "testuser"
	htmlURL := "https://github.com/test"

	tests := []struct {
		name     string
		comment  *github.PullRequestComment
		expected bool
	}{
		{
			name: "valid comment",
			comment: &github.PullRequestComment{
				Body:    &body,
				User:    &github.User{Login: &login},
				HTMLURL: &htmlURL,
			},
			expected: true,
		},
		{
			name:     "nil comment",
			comment:  nil,
			expected: false,
		},
		{
			name: "empty body",
			comment: &github.PullRequestComment{
				Body:    github.String(""),
				User:    &github.User{Login: &login},
				HTMLURL: &htmlURL,
			},
			expected: false,
		},
		{
			name: "nil user",
			comment: &github.PullRequestComment{
				Body:    &body,
				User:    nil,
				HTMLURL: &htmlURL,
			},
			expected: false,
		},
		{
			name: "empty login",
			comment: &github.PullRequestComment{
				Body:    &body,
				User:    &github.User{Login: github.String("")},
				HTMLURL: &htmlURL,
			},
			expected: false,
		},
		{
			name: "empty HTML URL",
			comment: &github.PullRequestComment{
				Body:    &body,
				User:    &github.User{Login: &login},
				HTMLURL: github.String(""),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidReviewComment(tt.comment)
			if result != tt.expected {
				t.Errorf("isValidReviewComment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProcessComment(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	url := "https://github.com/owner/repo/issues/1#comment"

	// Create a mock PR with author
	prAuthor := "prauthor"
	prNumber := 123
	owner := "owner"
	repo := "repo"

	pr := &github.PullRequest{
		Number: &prNumber,
		User:   &github.User{Login: &prAuthor},
		Base: &github.PullRequestBranch{
			Repo: &github.Repository{
				Owner: &github.User{Login: &owner},
				Name:  &repo,
			},
		},
	}

	tests := []struct {
		name           string
		body           string
		author         string
		expectGuidance bool
		expectError    bool
	}{
		{
			name:           "valid guidance from PR author",
			body:           "/rcs This is important guidance",
			author:         prAuthor,
			expectGuidance: true,
			expectError:    false,
		},
		{
			name:           "no guidance pattern",
			body:           "Just a regular comment",
			author:         "someone",
			expectGuidance: false,
			expectError:    false,
		},
		{
			name:           "invalid rcs pattern",
			body:           "Before text /rcs should not match",
			author:         "someone",
			expectGuidance: false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For this test, we can't easily mock the GitHub client
			// So we'll test the non-authorized path by using a nil client
			// This will cause isAuthorized to fail when fetching reviews

			guidance, err := processComment(ctx, tt.body, tt.author, now, url, nil, pr, prNumber, "test")

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil && tt.expectGuidance {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectGuidance && guidance == nil && !tt.expectError {
				t.Error("expected guidance but got nil")
			}
			if !tt.expectGuidance && guidance != nil {
				t.Error("expected no guidance but got one")
			}

			if guidance != nil {
				if guidance.Author != tt.author {
					t.Errorf("author = %s, want %s", guidance.Author, tt.author)
				}
				if guidance.CommentURL != url {
					t.Errorf("url = %s, want %s", guidance.CommentURL, url)
				}
			}
		})
	}
}

func TestFetchAllPaginated(t *testing.T) {
	ctx := context.Background()

	t.Run("single page", func(t *testing.T) {
		callCount := 0
		fetcher := func(ctx context.Context, opts *github.ListOptions) ([]string, *github.Response, error) {
			callCount++
			return []string{"item1", "item2"}, &github.Response{NextPage: 0}, nil
		}

		result, err := fetchAllPaginated(ctx, fetcher)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("len(result) = %d, want 2", len(result))
		}
		if callCount != 1 {
			t.Errorf("fetcher called %d times, want 1", callCount)
		}
	})

	t.Run("multiple pages", func(t *testing.T) {
		callCount := 0
		fetcher := func(ctx context.Context, opts *github.ListOptions) ([]string, *github.Response, error) {
			callCount++
			switch callCount {
			case 1:
				return []string{"item1", "item2"}, &github.Response{NextPage: 2}, nil
			case 2:
				return []string{"item3", "item4"}, &github.Response{NextPage: 3}, nil
			case 3:
				return []string{"item5"}, &github.Response{NextPage: 0}, nil
			default:
				return nil, nil, fmt.Errorf("unexpected call")
			}
		}

		result, err := fetchAllPaginated(ctx, fetcher)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 5 {
			t.Errorf("len(result) = %d, want 5", len(result))
		}
		if callCount != 3 {
			t.Errorf("fetcher called %d times, want 3", callCount)
		}
	})

	t.Run("error on fetch", func(t *testing.T) {
		fetcher := func(ctx context.Context, opts *github.ListOptions) ([]string, *github.Response, error) {
			return nil, nil, fmt.Errorf("API error")
		}

		result, err := fetchAllPaginated(ctx, fetcher)
		if err == nil {
			t.Error("expected error but got none")
		}
		if result != nil {
			t.Errorf("expected nil result on error, got %v", result)
		}
	})
}

func TestFetchUserGuidance_NilComparison(t *testing.T) {
	ctx := context.Background()
	cache := newPRCache()
	result, err := fetchUserGuidance(ctx, nil, "owner", "repo", nil, cache)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestFetchUserGuidance_EmptyCommits(t *testing.T) {
	ctx := context.Background()
	cache := newPRCache()
	comparison := &types.Comparison{
		Commits: []types.Commit{},
	}
	result, err := fetchUserGuidance(ctx, nil, "owner", "repo", comparison, cache)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestFetchUserGuidance_CommitsWithoutPRNumber(t *testing.T) {
	ctx := context.Background()
	cache := newPRCache()
	comparison := &types.Comparison{
		Commits: []types.Commit{
			{SHA: "abc123", PRNumber: 0},
			{SHA: "def456", PRNumber: 0},
		},
	}
	result, err := fetchUserGuidance(ctx, nil, "owner", "repo", comparison, cache)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestExtractUserGuidance_NilPR(t *testing.T) {
	ctx := context.Background()
	result, err := extractUserGuidance(ctx, nil, "owner", "repo", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestIsAuthorized_PRAuthor(t *testing.T) {
	ctx := context.Background()
	prAuthor := "author"
	prNumber := 123
	owner := "owner"
	repo := "repo"

	pr := &github.PullRequest{
		Number: &prNumber,
		User:   &github.User{Login: &prAuthor},
		Base: &github.PullRequestBranch{
			Repo: &github.Repository{
				Owner: &github.User{Login: &owner},
				Name:  &repo,
			},
		},
	}

	// Test: user is PR author
	result, err := isAuthorized(ctx, nil, pr, prAuthor)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for PR author, got false")
	}
}

func TestIsAuthorized_NotPRAuthor(t *testing.T) {
	// Testing non-author authorization requires a mock GitHub client
	// to properly test the review fetching logic
	t.Skip("Skipping - requires mock GitHub client to test review authorization")
}

func TestFetchUserGuidance_DuplicatePRs(t *testing.T) {
	// Testing deduplication with actual PR fetching requires a mock GitHub client
	t.Skip("Skipping - requires mock GitHub client to test deduplication logic")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr, 0))
}

func containsAt(s, substr string, offset int) bool {
	for i := offset; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
