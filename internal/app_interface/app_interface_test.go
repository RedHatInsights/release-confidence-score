package app_interface

import (
	"fmt"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/types"
	"testing"
	"time"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

func TestExtractDiffURLsFromBot(t *testing.T) {
	tests := []struct {
		name        string
		notes       []*gitlabapi.Note
		expected    []string
		expectError bool
	}{
		{
			name: "single URL from bot",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Diffs:\n- https://example.com/diff1",
				},
			},
			expected:    []string{"https://example.com/diff1"},
			expectError: false,
		},
		{
			name: "multiple URLs from bot",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Diffs:\n- https://example.com/diff1\n- https://example.com/diff2\n- https://example.com/diff3",
				},
			},
			expected:    []string{"https://example.com/diff1", "https://example.com/diff2", "https://example.com/diff3"},
			expectError: false,
		},
		{
			name: "bot comment with http URLs",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Diffs:\n- http://example.com/diff1\n- http://example.com/diff2",
				},
			},
			expected:    []string{"http://example.com/diff1", "http://example.com/diff2"},
			expectError: false,
		},
		{
			name: "bot comment with extra text",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Diffs:\nSome explanation here\n- https://example.com/diff1\nMore text\n- https://example.com/diff2",
				},
			},
			expected:    []string{"https://example.com/diff1", "https://example.com/diff2"},
			expectError: false,
		},
		{
			name: "no bot comments",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "other-user"},
					Body:   "Some comment",
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "bot comment without Diffs marker",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Some other bot comment\n- https://example.com/diff1",
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "bot comment with Diffs marker but no URLs",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Diffs:\nNo URLs here",
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "multiple notes, bot is not first",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "user1"},
					Body:   "Regular comment",
				},
				{
					Author: gitlabapi.NoteAuthor{Username: "user2"},
					Body:   "Another comment",
				},
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Diffs:\n- https://example.com/diff1",
				},
			},
			expected:    []string{"https://example.com/diff1"},
			expectError: false,
		},
		{
			name: "URLs without dash prefix not matched",
			notes: []*gitlabapi.Note{
				{
					Author: gitlabapi.NoteAuthor{Username: "devtools-bot"},
					Body:   "Diffs:\nhttps://example.com/diff1\n- https://example.com/diff2",
				},
			},
			expected:    []string{"https://example.com/diff2"},
			expectError: false,
		},
		{
			name:        "empty notes list",
			notes:       []*gitlabapi.Note{},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls, err := extractDiffURLsFromBot(tt.notes)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(urls) != len(tt.expected) {
				t.Errorf("Expected %d URLs, got %d", len(tt.expected), len(urls))
			}

			for i, url := range urls {
				if i >= len(tt.expected) {
					break
				}
				if url != tt.expected[i] {
					t.Errorf("URL %d: expected '%s', got '%s'", i, tt.expected[i], url)
				}
			}
		})
	}
}

func TestExtractUserGuidance(t *testing.T) {
	// Set up config for URL generation
	t.Setenv("RCS_GITHUB_TOKEN", "test-token")
	t.Setenv("RCS_GITLAB_BASE_URL", "https://gitlab.example.com")
	t.Setenv("RCS_GITLAB_TOKEN", "test-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "test-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "test-key")
	cfg, err := config.Load(true) // app-interface mode
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	mergeRequestIID := 123
	createdAt := time.Now()

	tests := []struct {
		name     string
		notes    []*gitlabapi.Note
		expected []types.UserGuidance
	}{
		{
			name: "single guidance",
			notes: []*gitlabapi.Note{
				{
					ID:        1,
					Author:    gitlabapi.NoteAuthor{Username: "alice"},
					Body:      "/rcs please review carefully",
					CreatedAt: &createdAt,
				},
			},
			expected: []types.UserGuidance{
				{
					Content:      "please review carefully",
					Author:       "alice",
					Date:         createdAt,
					CommentURL:   fmt.Sprintf("%s/%s/-/merge_requests/%d#note_%d", cfg.GitLabBaseURL, projectID, mergeRequestIID, 1),
					IsAuthorized: true,
				},
			},
		},
		{
			name: "multiple guidance from different users",
			notes: []*gitlabapi.Note{
				{
					ID:        1,
					Author:    gitlabapi.NoteAuthor{Username: "alice"},
					Body:      "/rcs first guidance",
					CreatedAt: &createdAt,
				},
				{
					ID:        2,
					Author:    gitlabapi.NoteAuthor{Username: "bob"},
					Body:      "/rcs second guidance",
					CreatedAt: &createdAt,
				},
			},
			expected: []types.UserGuidance{
				{
					Content:      "first guidance",
					Author:       "alice",
					Date:         createdAt,
					CommentURL:   fmt.Sprintf("%s/%s/-/merge_requests/%d#note_%d", cfg.GitLabBaseURL, projectID, mergeRequestIID, 1),
					IsAuthorized: true,
				},
				{
					Content:      "second guidance",
					Author:       "bob",
					Date:         createdAt,
					CommentURL:   fmt.Sprintf("%s/%s/-/merge_requests/%d#note_%d", cfg.GitLabBaseURL, projectID, mergeRequestIID, 2),
					IsAuthorized: true,
				},
			},
		},
		{
			name: "mixed notes with and without guidance",
			notes: []*gitlabapi.Note{
				{
					ID:        1,
					Author:    gitlabapi.NoteAuthor{Username: "alice"},
					Body:      "Regular comment",
					CreatedAt: &createdAt,
				},
				{
					ID:        2,
					Author:    gitlabapi.NoteAuthor{Username: "bob"},
					Body:      "/rcs important guidance",
					CreatedAt: &createdAt,
				},
				{
					ID:        3,
					Author:    gitlabapi.NoteAuthor{Username: "charlie"},
					Body:      "Another regular comment",
					CreatedAt: &createdAt,
				},
			},
			expected: []types.UserGuidance{
				{
					Content:      "important guidance",
					Author:       "bob",
					Date:         createdAt,
					CommentURL:   fmt.Sprintf("%s/%s/-/merge_requests/%d#note_%d", cfg.GitLabBaseURL, projectID, mergeRequestIID, 2),
					IsAuthorized: true,
				},
			},
		},
		{
			name: "multiline guidance",
			notes: []*gitlabapi.Note{
				{
					ID:        1,
					Author:    gitlabapi.NoteAuthor{Username: "alice"},
					Body:      "/rcs line 1\nline 2\nline 3",
					CreatedAt: &createdAt,
				},
			},
			expected: []types.UserGuidance{
				{
					Content:      "line 1\nline 2\nline 3",
					Author:       "alice",
					Date:         createdAt,
					CommentURL:   fmt.Sprintf("%s/%s/-/merge_requests/%d#note_%d", cfg.GitLabBaseURL, projectID, mergeRequestIID, 1),
					IsAuthorized: true,
				},
			},
		},
		{
			name: "no guidance in notes",
			notes: []*gitlabapi.Note{
				{
					ID:        1,
					Author:    gitlabapi.NoteAuthor{Username: "alice"},
					Body:      "Regular comment",
					CreatedAt: &createdAt,
				},
			},
			expected: []types.UserGuidance{},
		},
		{
			name: "skip note with nil CreatedAt",
			notes: []*gitlabapi.Note{
				{
					ID:        1,
					Author:    gitlabapi.NoteAuthor{Username: "alice"},
					Body:      "/rcs should be skipped",
					CreatedAt: nil,
				},
				{
					ID:        2,
					Author:    gitlabapi.NoteAuthor{Username: "bob"},
					Body:      "/rcs should be included",
					CreatedAt: &createdAt,
				},
			},
			expected: []types.UserGuidance{
				{
					Content:      "should be included",
					Author:       "bob",
					Date:         createdAt,
					CommentURL:   fmt.Sprintf("%s/%s/-/merge_requests/%d#note_%d", cfg.GitLabBaseURL, projectID, mergeRequestIID, 2),
					IsAuthorized: true,
				},
			},
		},
		{
			name:     "empty notes list",
			notes:    []*gitlabapi.Note{},
			expected: []types.UserGuidance{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guidance := extractUserGuidance(cfg, mergeRequestIID, tt.notes)

			if len(guidance) != len(tt.expected) {
				t.Errorf("Expected %d guidance items, got %d", len(tt.expected), len(guidance))
			}

			for i, g := range guidance {
				if i >= len(tt.expected) {
					break
				}
				exp := tt.expected[i]

				if g.Content != exp.Content {
					t.Errorf("Guidance %d: expected content '%s', got '%s'", i, exp.Content, g.Content)
				}
				if g.Author != exp.Author {
					t.Errorf("Guidance %d: expected author '%s', got '%s'", i, exp.Author, g.Author)
				}
				if !g.Date.Equal(exp.Date) {
					t.Errorf("Guidance %d: expected date '%v', got '%v'", i, exp.Date, g.Date)
				}
				if g.CommentURL != exp.CommentURL {
					t.Errorf("Guidance %d: expected URL '%s', got '%s'", i, exp.CommentURL, g.CommentURL)
				}
				if g.IsAuthorized != exp.IsAuthorized {
					t.Errorf("Guidance %d: expected IsAuthorized=%v, got %v", i, exp.IsAuthorized, g.IsAuthorized)
				}
			}
		})
	}
}

func TestPostReportToMR(t *testing.T) {
	// Note: This function now accepts a client as a parameter, making it much more testable.
	// To fully test this, you would need to:
	// 1. Create a mock GitLab client that implements the Notes.CreateMergeRequestNote interface
	// 2. Test success case: mock returns no error
	// 3. Test error case: mock returns error
	// 4. Verify the correct parameters are passed (projectID, mrIID, report body)
	//
	// For now, we document the test approach. A full implementation would require
	// a mocking library or custom test doubles.

	t.Run("function signature accepts client parameter", func(t *testing.T) {
		// Verify the function exists with the expected signature
		// This is a compile-time check that ensures the refactoring is correct
		var _ func(*gitlabapi.Client, string, int) error = PostReportToMR
	})
}
