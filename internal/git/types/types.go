package types

import (
	"regexp"
	"time"
)

// Comparison represents a git comparison between two refs, platform-agnostic
// Combines both raw diff data (files, stats) and enriched commit metadata (SHA, PR#, QE labels)
type Comparison struct {
	RepoURL string          // Repository URL (e.g., "https://github.com/owner/repo")
	DiffURL string          // Direct link to the comparison/diff
	Commits []Commit        // Commits in this comparison with full metadata
	Files   []FileChange    // Files changed in this comparison
	Stats   ComparisonStats // Statistics about the comparison
}

// FileChange represents a file that was changed in a comparison
type FileChange struct {
	Filename         string
	Status           string // added, modified, removed, renamed
	Additions        int
	Deletions        int
	Changes          int
	Patch            string
	PreviousFilename string // For renames
}

// ComparisonStats represents statistics about the comparison
type ComparisonStats struct {
	TotalFiles     int
	TotalAdditions int
	TotalDeletions int
	TotalChanges   int
}

// Repository represents basic repository information
type Repository struct {
	Owner         string
	Name          string
	DefaultBranch string
	URL           string
}

// Documentation represents repository documentation
type Documentation struct {
	MainDocContent  string
	MainDocFile     string
	LinkedDocs      map[string]string
	LinkedDocsOrder []string
	Repository      Repository
}

// UserGuidance represents a complete user guidance with metadata for reporting
type UserGuidance struct {
	Content      string    // The actual guidance content
	Author       string    // Platform username who posted it
	Date         time.Time // When it was posted
	CommentURL   string    // Direct link to the comment
	IsAuthorized bool      // Whether the author had permission to post
}

// Pre-compiled regex for user guidance parsing.
// Matches "/rcs <guidance text>" where guidance text can span multiple lines (case-insensitive).
var guidancePattern = regexp.MustCompile(`(?is)/rcs\s+(\S.*\S|\S)`)

// ParseUserGuidance extracts user guidance from text using a regex pattern.
// If the text starts with /rcs, everything after it is captured as guidance.
// Returns the guidance text and true if found, empty string and false otherwise.
func ParseUserGuidance(text string) (string, bool) {
	matches := guidancePattern.FindStringSubmatch(text)
	if len(matches) > 1 {
		guidance := matches[1]
		return guidance, true
	}

	return "", false
}

// Commit represents a single commit with enriched metadata
type Commit struct {
	SHA            string // Full commit SHA
	ShortSHA       string // Short SHA for display
	Message        string // Commit message (first line only)
	Author         string // Author name
	PRNumber       int    // Associated PR number (0 if none)
	QETestingLabel string // QE testing label status: "qe-tested", "needs-qe-testing", or empty
}
