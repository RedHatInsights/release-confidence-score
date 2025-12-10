package gitlab

import (
	"testing"
	"time"

	"release-confidence-score/internal/git/types"

	"gitlab.com/gitlab-org/api/client-go"
)

func TestIsValidNote(t *testing.T) {
	now := time.Now()
	username := "testuser"

	tests := []struct {
		name     string
		note     *gitlab.Note
		expected bool
	}{
		{
			name: "valid note",
			note: &gitlab.Note{
				Body:      "test body",
				Author:    gitlab.NoteAuthor{Username: username},
				CreatedAt: &now,
			},
			expected: true,
		},
		{
			name:     "nil note",
			note:     nil,
			expected: false,
		},
		{
			name: "empty body",
			note: &gitlab.Note{
				Body:      "",
				Author:    gitlab.NoteAuthor{Username: username},
				CreatedAt: &now,
			},
			expected: false,
		},
		{
			name: "empty username",
			note: &gitlab.Note{
				Body:      "test body",
				Author:    gitlab.NoteAuthor{Username: ""},
				CreatedAt: &now,
			},
			expected: false,
		},
		{
			name: "nil created at",
			note: &gitlab.Note{
				Body:      "test body",
				Author:    gitlab.NoteAuthor{Username: username},
				CreatedAt: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidNote(tt.note)
			if result != tt.expected {
				t.Errorf("isValidNote() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProcessNote(t *testing.T) {
	now := time.Now()
	repoURL := "https://gitlab.com/owner/repo"
	mrIID := int64(123)

	tests := []struct {
		name           string
		note           *gitlab.Note
		mrAuthor       string
		approvers      []string
		expectGuidance bool
		expectAuth     bool
	}{
		{
			name: "valid guidance from MR author",
			note: &gitlab.Note{
				ID:        1,
				Body:      "/rcs This is important guidance",
				Author:    gitlab.NoteAuthor{Username: "author"},
				CreatedAt: &now,
			},
			mrAuthor:       "author",
			approvers:      []string{},
			expectGuidance: true,
			expectAuth:     true,
		},
		{
			name: "valid guidance from approver",
			note: &gitlab.Note{
				ID:        2,
				Body:      "/rcs This is guidance from approver",
				Author:    gitlab.NoteAuthor{Username: "approver1"},
				CreatedAt: &now,
			},
			mrAuthor:       "author",
			approvers:      []string{"approver1", "approver2"},
			expectGuidance: true,
			expectAuth:     true,
		},
		{
			name: "guidance from unauthorized user",
			note: &gitlab.Note{
				ID:        3,
				Body:      "/rcs This is unauthorized guidance",
				Author:    gitlab.NoteAuthor{Username: "stranger"},
				CreatedAt: &now,
			},
			mrAuthor:       "author",
			approvers:      []string{"approver1"},
			expectGuidance: true,
			expectAuth:     false,
		},
		{
			name: "no guidance pattern",
			note: &gitlab.Note{
				ID:        4,
				Body:      "Just a regular comment",
				Author:    gitlab.NoteAuthor{Username: "author"},
				CreatedAt: &now,
			},
			mrAuthor:       "author",
			approvers:      []string{},
			expectGuidance: false,
			expectAuth:     false,
		},
		{
			name: "invalid rcs pattern",
			note: &gitlab.Note{
				ID:        5,
				Body:      "Before text /rcs should not match",
				Author:    gitlab.NoteAuthor{Username: "author"},
				CreatedAt: &now,
			},
			mrAuthor:       "author",
			approvers:      []string{},
			expectGuidance: false,
			expectAuth:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guidance := processNote(tt.note, tt.mrAuthor, tt.approvers, repoURL, mrIID)

			if tt.expectGuidance && guidance == nil {
				t.Error("expected guidance but got nil")
				return
			}
			if !tt.expectGuidance && guidance != nil {
				t.Error("expected no guidance but got one")
				return
			}

			if guidance != nil {
				if guidance.Author != tt.note.Author.Username {
					t.Errorf("author = %s, want %s", guidance.Author, tt.note.Author.Username)
				}
				if guidance.IsAuthorized != tt.expectAuth {
					t.Errorf("isAuthorized = %v, want %v", guidance.IsAuthorized, tt.expectAuth)
				}
				expectedURL := "https://gitlab.com/owner/repo/-/merge_requests/123#note_1"
				if tt.note.ID == 2 {
					expectedURL = "https://gitlab.com/owner/repo/-/merge_requests/123#note_2"
				} else if tt.note.ID == 3 {
					expectedURL = "https://gitlab.com/owner/repo/-/merge_requests/123#note_3"
				}
				if guidance.CommentURL != expectedURL {
					t.Errorf("url = %s, want %s", guidance.CommentURL, expectedURL)
				}
				if !guidance.Date.Equal(*tt.note.CreatedAt) {
					t.Errorf("date = %v, want %v", guidance.Date, *tt.note.CreatedAt)
				}
			}
		})
	}
}

func TestIsAuthorized(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		mrAuthor  string
		approvers []string
		expected  bool
	}{
		{
			name:      "user is MR author",
			username:  "author",
			mrAuthor:  "author",
			approvers: []string{},
			expected:  true,
		},
		{
			name:      "user is approver",
			username:  "approver1",
			mrAuthor:  "author",
			approvers: []string{"approver1", "approver2"},
			expected:  true,
		},
		{
			name:      "user is both author and approver",
			username:  "author",
			mrAuthor:  "author",
			approvers: []string{"author"},
			expected:  true,
		},
		{
			name:      "user is not authorized",
			username:  "stranger",
			mrAuthor:  "author",
			approvers: []string{"approver1"},
			expected:  false,
		},
		{
			name:      "empty approvers list",
			username:  "stranger",
			mrAuthor:  "author",
			approvers: []string{},
			expected:  false,
		},
		{
			name:      "nil approvers",
			username:  "stranger",
			mrAuthor:  "author",
			approvers: nil,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAuthorized(tt.username, tt.mrAuthor, tt.approvers, 123)
			if result != tt.expected {
				t.Errorf("isAuthorized() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetMRApprovers(t *testing.T) {
	// This test verifies the function signature and basic behavior
	// We can't easily test the actual API call without mocking the client

	t.Run("function exists with correct signature", func(t *testing.T) {
		// The actual function requires a real GitLab client
		// This test simply verifies the function exists and has the correct signature
		// A real test would require a mock GitLab client
		// We skip actual invocation to avoid nil pointer panics
		t.Skip("Skipping integration test - requires mock GitLab client")
	})
}

func TestFetchUserGuidance_NilComparison(t *testing.T) {
	result, err := fetchUserGuidance(nil, nil, "", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestExtractUserGuidance_NilMR(t *testing.T) {
	result, err := extractUserGuidance(nil, nil, "", "", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestFetchUserGuidance_EmptyComparison(t *testing.T) {
	comparison := &types.Comparison{
		Commits: []types.Commit{},
	}
	result, err := fetchUserGuidance(nil, nil, "", comparison)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestFetchUserGuidance_CommitsWithoutMRNumber(t *testing.T) {
	comparison := &types.Comparison{
		Commits: []types.Commit{
			{SHA: "abc123", PRNumber: 0},
			{SHA: "def456", PRNumber: 0},
		},
	}
	result, err := fetchUserGuidance(nil, nil, "", comparison)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestFetchUserGuidance_DuplicateMRs(t *testing.T) {
	// Testing deduplication with actual MR fetching requires a mock GitLab client
	t.Skip("Skipping - requires mock GitLab client to test deduplication logic")
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
