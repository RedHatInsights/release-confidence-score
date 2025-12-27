package shared

import (
	"testing"

	"github.com/google/go-github/v80/github"
)

func TestNewRESTClient(t *testing.T) {
	client := NewRESTClient("test-token")
	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestConvertFile(t *testing.T) {
	tests := []struct {
		name     string
		file     *github.CommitFile
		expected string
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
			result := ConvertFile(tt.file)
			if result.Filename != tt.expected {
				t.Errorf("ConvertFile().Filename = %q, want %q", result.Filename, tt.expected)
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

	result := ConvertFile(file)

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
			result := ConvertFiles(tt.files)
			if len(result) != tt.expected {
				t.Errorf("len(ConvertFiles()) = %d, want %d", len(result), tt.expected)
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
			result := CalculateStats(tt.files)
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

func TestBuildBasicCommitEntry(t *testing.T) {
	tests := []struct {
		name        string
		commit      *github.RepositoryCommit
		wantSHA     string
		wantShort   string
		wantMessage string
		wantAuthor  string
	}{
		{
			name: "complete commit",
			commit: &github.RepositoryCommit{
				SHA: github.Ptr("abc123def456789012345678901234567890abcd"),
				Commit: &github.Commit{
					Message: github.Ptr("Fix bug in parser"),
					Author:  &github.CommitAuthor{Name: github.Ptr("John Doe")},
				},
			},
			wantSHA:     "abc123def456789012345678901234567890abcd",
			wantShort:   "abc123de",
			wantMessage: "Fix bug in parser",
			wantAuthor:  "John Doe",
		},
		{
			name: "multiline message",
			commit: &github.RepositoryCommit{
				SHA: github.Ptr("def456abc789012345678901234567890abcdef12"),
				Commit: &github.Commit{
					Message: github.Ptr("Add feature\n\nDetailed description here"),
					Author:  &github.CommitAuthor{Name: github.Ptr("Jane Smith")},
				},
			},
			wantSHA:     "def456abc789012345678901234567890abcdef12",
			wantShort:   "def456ab",
			wantMessage: "Add feature",
			wantAuthor:  "Jane Smith",
		},
		{
			name: "empty message and author",
			commit: &github.RepositoryCommit{
				SHA: github.Ptr("1234567890abcdef1234567890abcdef12345678"),
				Commit: &github.Commit{
					Message: github.Ptr(""),
					Author:  &github.CommitAuthor{Name: github.Ptr("")},
				},
			},
			wantSHA:     "1234567890abcdef1234567890abcdef12345678",
			wantShort:   "12345678",
			wantMessage: "No message",
			wantAuthor:  "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildBasicCommitEntry(tt.commit)

			if result.SHA != tt.wantSHA {
				t.Errorf("SHA = %q, want %q", result.SHA, tt.wantSHA)
			}
			if result.ShortSHA != tt.wantShort {
				t.Errorf("ShortSHA = %q, want %q", result.ShortSHA, tt.wantShort)
			}
			if result.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", result.Message, tt.wantMessage)
			}
			if result.Author != tt.wantAuthor {
				t.Errorf("Author = %q, want %q", result.Author, tt.wantAuthor)
			}
		})
	}
}
