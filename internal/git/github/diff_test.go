package github

import (
	"testing"

	"github.com/google/go-github/v80/github"
)

func TestExtractQELabel(t *testing.T) {
	tests := []struct {
		name     string
		pr       *github.PullRequest
		expected string
	}{
		{
			name:     "nil PR",
			pr:       nil,
			expected: "",
		},
		{
			name:     "no labels",
			pr:       &github.PullRequest{Labels: []*github.Label{}},
			expected: "",
		},
		{
			name: "qe-tested label",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("rcs/qe-tested")},
				},
			},
			expected: "rcs/qe-tested",
		},
		{
			name: "needs-qe-testing label",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("rcs/needs-qe-testing")},
				},
			},
			expected: "rcs/needs-qe-testing",
		},
		{
			name: "both labels - qe-tested wins",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("rcs/needs-qe-testing")},
					{Name: github.Ptr("rcs/qe-tested")},
				},
			},
			expected: "rcs/qe-tested",
		},
		{
			name: "unrelated labels",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("bug")},
					{Name: github.Ptr("enhancement")},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractQELabel(tt.pr)
			if result != tt.expected {
				t.Errorf("extractQELabel() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertFile(t *testing.T) {
	tests := []struct {
		name     string
		file     *github.CommitFile
		expected string // Check filename as proxy for correct conversion
	}{
		{
			name:     "nil file",
			file:     nil,
			expected: "",
		},
		{
			name: "complete file",
			file: &github.CommitFile{
				Filename:         github.Ptr("src/main.go"),
				Status:           github.Ptr("modified"),
				Additions:        github.Ptr(10),
				Deletions:        github.Ptr(5),
				Changes:          github.Ptr(15),
				Patch:            github.Ptr("@@ -1,5 +1,10 @@"),
				PreviousFilename: github.Ptr("src/old.go"),
			},
			expected: "src/main.go",
		},
		{
			name: "file with nil fields",
			file: &github.CommitFile{
				Filename: github.Ptr("test.go"),
			},
			expected: "test.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertFile(tt.file)
			if result.Filename != tt.expected {
				t.Errorf("convertFile().Filename = %q, want %q", result.Filename, tt.expected)
			}
		})
	}
}

func TestConvertFileFields(t *testing.T) {
	file := &github.CommitFile{
		Filename:         github.Ptr("src/main.go"),
		Status:           github.Ptr("modified"),
		Additions:        github.Ptr(10),
		Deletions:        github.Ptr(5),
		Changes:          github.Ptr(15),
		Patch:            github.Ptr("@@ -1,5 +1,10 @@"),
		PreviousFilename: github.Ptr("src/old.go"),
	}

	result := convertFile(file)

	if result.Filename != "src/main.go" {
		t.Errorf("Filename = %q, want %q", result.Filename, "src/main.go")
	}
	if result.Status != "modified" {
		t.Errorf("Status = %q, want %q", result.Status, "modified")
	}
	if result.Additions != 10 {
		t.Errorf("Additions = %d, want %d", result.Additions, 10)
	}
	if result.Deletions != 5 {
		t.Errorf("Deletions = %d, want %d", result.Deletions, 5)
	}
	if result.Changes != 15 {
		t.Errorf("Changes = %d, want %d", result.Changes, 15)
	}
	if result.Patch != "@@ -1,5 +1,10 @@" {
		t.Errorf("Patch = %q, want %q", result.Patch, "@@ -1,5 +1,10 @@")
	}
	if result.PreviousFilename != "src/old.go" {
		t.Errorf("PreviousFilename = %q, want %q", result.PreviousFilename, "src/old.go")
	}
}

func TestConvertFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    []*github.CommitFile
		expected int
	}{
		{
			name:     "nil files",
			files:    nil,
			expected: 0,
		},
		{
			name:     "empty files",
			files:    []*github.CommitFile{},
			expected: 0,
		},
		{
			name: "multiple files",
			files: []*github.CommitFile{
				{Filename: github.Ptr("a.go")},
				{Filename: github.Ptr("b.go")},
				{Filename: github.Ptr("c.go")},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertFiles(tt.files)
			if len(result) != tt.expected {
				t.Errorf("len(convertFiles()) = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name              string
		files             []*github.CommitFile
		expectedFiles     int
		expectedAdditions int
		expectedDeletions int
	}{
		{
			name:              "empty files",
			files:             []*github.CommitFile{},
			expectedFiles:     0,
			expectedAdditions: 0,
			expectedDeletions: 0,
		},
		{
			name: "single file",
			files: []*github.CommitFile{
				{Additions: github.Ptr(10), Deletions: github.Ptr(5)},
			},
			expectedFiles:     1,
			expectedAdditions: 10,
			expectedDeletions: 5,
		},
		{
			name: "multiple files",
			files: []*github.CommitFile{
				{Additions: github.Ptr(10), Deletions: github.Ptr(5)},
				{Additions: github.Ptr(20), Deletions: github.Ptr(3)},
				{Additions: github.Ptr(5), Deletions: github.Ptr(2)},
			},
			expectedFiles:     3,
			expectedAdditions: 35,
			expectedDeletions: 10,
		},
		{
			name: "files with nil stats",
			files: []*github.CommitFile{
				{Additions: github.Ptr(10)},
				{Deletions: github.Ptr(5)},
				{},
			},
			expectedFiles:     3,
			expectedAdditions: 10,
			expectedDeletions: 5,
		},
		{
			name: "with nil file element",
			files: []*github.CommitFile{
				{Additions: github.Ptr(10), Deletions: github.Ptr(5)},
				nil,
				{Additions: github.Ptr(5), Deletions: github.Ptr(2)},
			},
			expectedFiles:     3,
			expectedAdditions: 15,
			expectedDeletions: 7,
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
