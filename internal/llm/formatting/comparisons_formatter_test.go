package formatting

import (
	"strings"
	"testing"

	"release-confidence-score/internal/git/types"
)

func TestFormatComparisons(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		result := FormatComparisons([]*types.Comparison{})
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("nil list", func(t *testing.T) {
		result := FormatComparisons(nil)
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("nil comparison in list", func(t *testing.T) {
		result := FormatComparisons([]*types.Comparison{nil})
		if result != "" {
			t.Errorf("expected empty string for nil comparison, got %q", result)
		}
	})

	t.Run("comparison with no commits", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{},
		}
		result := FormatComparisons([]*types.Comparison{comparison})
		if result != "" {
			t.Errorf("expected empty string for comparison with no commits, got %q", result)
		}
	})

	t.Run("single comparison basic", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{
					Message: "Add feature",
					Author:  "Alice",
				},
			},
			Files: []types.FileChange{
				{
					Filename:  "file.go",
					Status:    "modified",
					Additions: 10,
					Deletions: 5,
					Patch:     "diff content",
				},
			},
			Stats: types.ComparisonStats{
				TotalFiles:     1,
				TotalAdditions: 10,
				TotalDeletions: 5,
			},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		// Check key components
		if !strings.Contains(result, "Repository: https://github.com/test/repo") {
			t.Error("missing repository header")
		}
		if !strings.Contains(result, "Add feature (Alice)") {
			t.Error("missing commit with author")
		}
		if !strings.Contains(result, "file.go: modified +10/-5") {
			t.Error("missing file with stats and status")
		}
		if !strings.Contains(result, "Total: 1 files, +10/-5 lines") {
			t.Error("missing total stats")
		}
		if !strings.Contains(result, "Diffs:") {
			t.Error("missing Diffs section")
		}
		if !strings.Contains(result, "diff content") {
			t.Error("missing diff content")
		}
	})

	t.Run("multiple comparisons", func(t *testing.T) {
		comparisons := []*types.Comparison{
			{
				RepoURL: "https://github.com/test/repo1",
				Commits: []types.Commit{
					{Message: "Commit 1", Author: "Alice"},
				},
				Files: []types.FileChange{
					{Filename: "a.go", Status: "added", Additions: 5, Patch: "diff a"},
				},
				Stats: types.ComparisonStats{TotalFiles: 1, TotalAdditions: 5},
			},
			{
				RepoURL: "https://github.com/test/repo2",
				Commits: []types.Commit{
					{Message: "Commit 2", Author: "Bob"},
				},
				Files: []types.FileChange{
					{Filename: "b.go", Status: "modified", Deletions: 3, Patch: "diff b"},
				},
				Stats: types.ComparisonStats{TotalFiles: 1, TotalDeletions: 3},
			},
		}

		result := FormatComparisons(comparisons)

		if !strings.Contains(result, "=== Diff 1: https://github.com/test/repo1 ===") {
			t.Error("missing first diff header")
		}
		if !strings.Contains(result, "=== Diff 2: https://github.com/test/repo2 ===") {
			t.Error("missing second diff header")
		}
		if !strings.Contains(result, "Commit 1 (Alice)") {
			t.Error("missing first commit")
		}
		if !strings.Contains(result, "Commit 2 (Bob)") {
			t.Error("missing second commit")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{Message: "Rename file", Author: "Alice"},
			},
			Files: []types.FileChange{
				{
					Filename:         "new.go",
					PreviousFilename: "old.go",
					Status:           "renamed",
					Additions:        5,
					Deletions:        2,
					Patch:            "diff content",
				},
			},
			Stats: types.ComparisonStats{TotalFiles: 1, TotalAdditions: 5, TotalDeletions: 2},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		if !strings.Contains(result, "new.go (renamed from old.go): renamed +5/-2") {
			t.Error("missing rename information in file list")
		}
	})

	t.Run("file without patch", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{Message: "Add binary", Author: "Alice"},
			},
			Files: []types.FileChange{
				{
					Filename:  "binary.exe",
					Status:    "added",
					Additions: 0,
					Deletions: 0,
					Patch:     "", // No patch for binary
				},
			},
			Stats: types.ComparisonStats{TotalFiles: 1},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		if !strings.Contains(result, "binary.exe: added +0/-0") {
			t.Error("missing binary file in file list")
		}
		if strings.Contains(result, "Diffs:") {
			t.Error("should not show Diffs section when no files have patches")
		}
	})

	t.Run("mixed files with and without patches", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{Message: "Mixed changes", Author: "Alice"},
			},
			Files: []types.FileChange{
				{Filename: "code.go", Status: "modified", Additions: 10, Patch: "diff code"},
				{Filename: "binary.exe", Status: "added", Additions: 0, Patch: ""},
				{Filename: "data.go", Status: "added", Additions: 5, Patch: "diff data"},
			},
			Stats: types.ComparisonStats{TotalFiles: 3, TotalAdditions: 15},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		// All files should appear in the file list
		if !strings.Contains(result, "code.go: modified +10/-0") {
			t.Error("missing code.go in file list")
		}
		if !strings.Contains(result, "binary.exe: added +0/-0") {
			t.Error("missing binary.exe in file list")
		}
		if !strings.Contains(result, "data.go: added +5/-0") {
			t.Error("missing data.go in file list")
		}

		// Diffs section should only show files with patches
		if !strings.Contains(result, "Diffs:") {
			t.Error("missing Diffs section")
		}
		if !strings.Contains(result, "diff code") {
			t.Error("missing code.go diff")
		}
		if !strings.Contains(result, "diff data") {
			t.Error("missing data.go diff")
		}
	})

	t.Run("unknown author name", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{Message: "Commit", Author: ""},
			},
			Files: []types.FileChange{
				{Filename: "file.go", Status: "added", Patch: "diff"},
			},
			Stats: types.ComparisonStats{TotalFiles: 1},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		if !strings.Contains(result, "Commit (Unknown)") {
			t.Error("expected 'Unknown' for empty author name")
		}
	})

	t.Run("multiline commit message", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{
					Message: "First line\nSecond line\nThird line",
					Author:  "Alice",
				},
			},
			Files: []types.FileChange{
				{Filename: "file.go", Status: "added", Patch: "diff"},
			},
			Stats: types.ComparisonStats{TotalFiles: 1},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		if !strings.Contains(result, "First line (Alice)") {
			t.Error("expected only first line of commit message")
		}
		if strings.Contains(result, "Second line") {
			t.Error("should not include second line of commit message")
		}
	})

	t.Run("empty commit message", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{Message: "", Author: "Alice"},
			},
			Files: []types.FileChange{
				{Filename: "file.go", Status: "added", Patch: "diff"},
			},
			Stats: types.ComparisonStats{TotalFiles: 1},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		if !strings.Contains(result, " (Alice)") {
			t.Error("expected author name even with empty message")
		}
	})

	t.Run("multiple commits", func(t *testing.T) {
		comparison := &types.Comparison{
			RepoURL: "https://github.com/test/repo",
			Commits: []types.Commit{
				{Message: "First commit", Author: "Alice"},
				{Message: "Second commit", Author: "Bob"},
				{Message: "Third commit", Author: "Charlie"},
			},
			Files: []types.FileChange{
				{Filename: "file.go", Status: "modified", Patch: "diff"},
			},
			Stats: types.ComparisonStats{TotalFiles: 1},
		}

		result := FormatComparisons([]*types.Comparison{comparison})

		if !strings.Contains(result, "First commit (Alice)") {
			t.Error("missing first commit")
		}
		if !strings.Contains(result, "Second commit (Bob)") {
			t.Error("missing second commit")
		}
		if !strings.Contains(result, "Third commit (Charlie)") {
			t.Error("missing third commit")
		}
	})
}
