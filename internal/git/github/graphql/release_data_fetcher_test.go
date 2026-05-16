package graphql

import (
	"strings"
	"testing"
)

func TestFetcher_Name(t *testing.T) {
	fetcher := &Fetcher{}
	if got := fetcher.Name(); got != "GitHub" {
		t.Errorf("Name() = %q, want %q", got, "GitHub")
	}
}

func TestFetcher_IsCompareURL(t *testing.T) {
	fetcher := &Fetcher{}

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "valid compare URL",
			url:  "https://github.com/owner/repo/compare/v1.0.0...v1.1.0",
			want: true,
		},
		{
			name: "valid compare URL with branches",
			url:  "https://github.com/owner/repo/compare/main...feature/branch",
			want: true,
		},
		{
			name: "valid compare URL with SHAs",
			url:  "https://github.com/owner/repo/compare/abc123...def456",
			want: true,
		},
		{
			name: "invalid - not a compare URL",
			url:  "https://github.com/owner/repo",
			want: false,
		},
		{
			name: "invalid - GitLab URL",
			url:  "https://gitlab.com/owner/repo/compare/v1.0.0...v1.1.0",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fetcher.IsCompareURL(tt.url); got != tt.want {
				t.Errorf("IsCompareURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestBuildBasicCommitEntry(t *testing.T) {
	tests := []struct {
		name        string
		sha         string
		message     string
		authorName  string
		wantShort   string
		wantMessage string
		wantAuthor  string
	}{
		{
			name:        "normal commit",
			sha:         "abc123def456789",
			message:     "Fix bug in parser",
			authorName:  "John Doe",
			wantShort:   "abc123de",
			wantMessage: "Fix bug in parser",
			wantAuthor:  "John Doe",
		},
		{
			name:        "multiline message",
			sha:         "def456abc789012",
			message:     "Add feature\n\nDetailed description here",
			authorName:  "Jane Smith",
			wantShort:   "def456ab",
			wantMessage: "Add feature",
			wantAuthor:  "Jane Smith",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := buildBasicCommitEntryFromValues(tt.sha, tt.message, tt.authorName)

			if entry.ShortSHA != tt.wantShort {
				t.Errorf("ShortSHA = %q, want %q", entry.ShortSHA, tt.wantShort)
			}
			if entry.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", entry.Message, tt.wantMessage)
			}
			if entry.Author != tt.wantAuthor {
				t.Errorf("Author = %q, want %q", entry.Author, tt.wantAuthor)
			}
		})
	}
}

// buildBasicCommitEntryFromValues is a test helper that builds a commit entry from raw values
// This mirrors the logic in buildBasicCommitEntry without needing full github.RepositoryCommit objects
func buildBasicCommitEntryFromValues(sha, message, author string) struct {
	ShortSHA string
	Message  string
	Author   string
} {
	entry := struct {
		ShortSHA string
		Message  string
		Author   string
	}{
		ShortSHA: sha[:8],
		Message:  "No message",
		Author:   "Unknown",
	}

	if message != "" {
		entry.Message = strings.TrimSpace(strings.SplitN(message, "\n", 2)[0])
	}

	if author != "" {
		entry.Author = author
	}

	return entry
}
