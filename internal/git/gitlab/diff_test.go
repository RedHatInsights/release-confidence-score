package gitlab

import (
	"testing"

	"release-confidence-score/internal/git/types"

	"gitlab.com/gitlab-org/api/client-go"
)

func TestExtractQELabel(t *testing.T) {
	tests := []struct {
		name     string
		mr       *gitlab.MergeRequest
		expected string
	}{
		{
			name:     "nil MR",
			mr:       nil,
			expected: "",
		},
		{
			name:     "no labels",
			mr:       &gitlab.MergeRequest{BasicMergeRequest: gitlab.BasicMergeRequest{Labels: gitlab.Labels{}}},
			expected: "",
		},
		{
			name: "qe-tested label",
			mr: &gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{Labels: gitlab.Labels{"rcs/qe-tested"}},
			},
			expected: "rcs/qe-tested",
		},
		{
			name: "needs-qe-testing label",
			mr: &gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{Labels: gitlab.Labels{"rcs/needs-qe-testing"}},
			},
			expected: "rcs/needs-qe-testing",
		},
		{
			name: "both labels - qe-tested wins",
			mr: &gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{Labels: gitlab.Labels{"rcs/needs-qe-testing", "rcs/qe-tested"}},
			},
			expected: "rcs/qe-tested",
		},
		{
			name: "unrelated labels",
			mr: &gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{Labels: gitlab.Labels{"bug", "enhancement"}},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractQELabel(tt.mr)
			if result != tt.expected {
				t.Errorf("extractQELabel() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertDiff(t *testing.T) {
	tests := []struct {
		name           string
		diff           *gitlab.Diff
		expectedFile   string
		expectedStatus string
	}{
		{
			name:           "nil diff",
			diff:           nil,
			expectedFile:   "",
			expectedStatus: "",
		},
		{
			name: "new file",
			diff: &gitlab.Diff{
				NewPath: "src/new.go",
				OldPath: "",
				NewFile: true,
				Diff:    "+package main",
			},
			expectedFile:   "src/new.go",
			expectedStatus: "added",
		},
		{
			name: "deleted file",
			diff: &gitlab.Diff{
				NewPath:     "",
				OldPath:     "src/old.go",
				DeletedFile: true,
				Diff:        "-package main",
			},
			expectedFile:   "",
			expectedStatus: "removed",
		},
		{
			name: "renamed file",
			diff: &gitlab.Diff{
				NewPath:     "src/new.go",
				OldPath:     "src/old.go",
				RenamedFile: true,
				Diff:        "",
			},
			expectedFile:   "src/new.go",
			expectedStatus: "renamed",
		},
		{
			name: "modified file",
			diff: &gitlab.Diff{
				NewPath: "src/main.go",
				OldPath: "src/main.go",
				Diff:    "@@ -1,5 +1,10 @@\n-old\n+new",
			},
			expectedFile:   "src/main.go",
			expectedStatus: "modified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertDiff(tt.diff)
			if result.Filename != tt.expectedFile {
				t.Errorf("Filename = %q, want %q", result.Filename, tt.expectedFile)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.expectedStatus)
			}
		})
	}
}

func TestConvertDiffStats(t *testing.T) {
	diff := &gitlab.Diff{
		NewPath: "src/main.go",
		OldPath: "src/main.go",
		Diff:    "@@ -1,5 +1,10 @@\n-old line 1\n-old line 2\n+new line 1\n+new line 2\n+new line 3",
	}

	result := convertDiff(diff)

	if result.Additions != 3 {
		t.Errorf("Additions = %d, want %d", result.Additions, 3)
	}
	if result.Deletions != 2 {
		t.Errorf("Deletions = %d, want %d", result.Deletions, 2)
	}
	if result.Changes != 5 {
		t.Errorf("Changes = %d, want %d", result.Changes, 5)
	}
}

func TestConvertDiffs(t *testing.T) {
	tests := []struct {
		name     string
		diffs    []*gitlab.Diff
		expected int
	}{
		{
			name:     "nil diffs",
			diffs:    nil,
			expected: 0,
		},
		{
			name:     "empty diffs",
			diffs:    []*gitlab.Diff{},
			expected: 0,
		},
		{
			name: "multiple diffs",
			diffs: []*gitlab.Diff{
				{NewPath: "a.go"},
				{NewPath: "b.go"},
				{NewPath: "c.go"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertDiffs(tt.diffs)
			if len(result) != tt.expected {
				t.Errorf("len(convertDiffs()) = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestParsePatchStats(t *testing.T) {
	tests := []struct {
		name              string
		patch             string
		expectedAdditions int
		expectedDeletions int
	}{
		{
			name:              "empty patch",
			patch:             "",
			expectedAdditions: 0,
			expectedDeletions: 0,
		},
		{
			name:              "only additions",
			patch:             "+line1\n+line2\n+line3",
			expectedAdditions: 3,
			expectedDeletions: 0,
		},
		{
			name:              "only deletions",
			patch:             "-line1\n-line2",
			expectedAdditions: 0,
			expectedDeletions: 2,
		},
		{
			name:              "mixed changes",
			patch:             "@@ -1,5 +1,10 @@\n-old\n+new\n context\n-removed\n+added1\n+added2",
			expectedAdditions: 3,
			expectedDeletions: 2,
		},
		{
			name:              "skip diff headers",
			patch:             "--- a/file.go\n+++ b/file.go\n@@ -1,5 +1,10 @@\n-old\n+new",
			expectedAdditions: 1,
			expectedDeletions: 1,
		},
		{
			name:              "empty lines in patch",
			patch:             "+line1\n\n+line2\n\n-line3",
			expectedAdditions: 2,
			expectedDeletions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			additions, deletions := parsePatchStats(tt.patch)
			if additions != tt.expectedAdditions {
				t.Errorf("additions = %d, want %d", additions, tt.expectedAdditions)
			}
			if deletions != tt.expectedDeletions {
				t.Errorf("deletions = %d, want %d", deletions, tt.expectedDeletions)
			}
		})
	}
}

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name              string
		files             []types.FileChange
		expectedFiles     int
		expectedAdditions int
		expectedDeletions int
	}{
		{
			name:              "empty files",
			files:             []types.FileChange{},
			expectedFiles:     0,
			expectedAdditions: 0,
			expectedDeletions: 0,
		},
		{
			name: "single file",
			files: []types.FileChange{
				{Additions: 2, Deletions: 1},
			},
			expectedFiles:     1,
			expectedAdditions: 2,
			expectedDeletions: 1,
		},
		{
			name: "multiple files",
			files: []types.FileChange{
				{Additions: 2, Deletions: 0},
				{Additions: 0, Deletions: 3},
				{Additions: 1, Deletions: 1},
			},
			expectedFiles:     3,
			expectedAdditions: 3,
			expectedDeletions: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateStats(tt.files)
			if result.TotalFiles != tt.expectedFiles {
				t.Errorf("TotalFiles = %d, want %d", result.TotalFiles, tt.expectedFiles)
			}
			if result.TotalAdditions != tt.expectedAdditions {
				t.Errorf("TotalAdditions = %d, want %d", result.TotalAdditions, tt.expectedAdditions)
			}
			if result.TotalDeletions != tt.expectedDeletions {
				t.Errorf("TotalDeletions = %d, want %d", result.TotalDeletions, tt.expectedDeletions)
			}
			if result.TotalChanges != tt.expectedAdditions+tt.expectedDeletions {
				t.Errorf("TotalChanges = %d, want %d", result.TotalChanges, tt.expectedAdditions+tt.expectedDeletions)
			}
		})
	}
}

func TestCacheKey(t *testing.T) {
	tests := []struct {
		projectPath string
		mrIID       int64
		expected    string
	}{
		{"group/project", 123, "group/project/123"},
		{"org/sub/repo", 456, "org/sub/repo/456"},
		{"simple", 1, "simple/1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := cacheKey(tt.projectPath, tt.mrIID)
			if result != tt.expected {
				t.Errorf("cacheKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNewMRCache(t *testing.T) {
	cache := newMRCache()

	if cache == nil {
		t.Fatal("newMRCache() returned nil")
	}
	if cache.commitToMR == nil {
		t.Error("commitToMR map is nil")
	}
	if cache.mergeRequests == nil {
		t.Error("mergeRequests map is nil")
	}
}
