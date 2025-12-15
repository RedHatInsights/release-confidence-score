package types

import (
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

// ComparisonStats represents statistics about the comparison
type ComparisonStats struct {
	TotalFiles     int
	TotalAdditions int
	TotalDeletions int
	TotalChanges   int
}

// Commit represents a single commit with enriched metadata
type Commit struct {
	SHA            string // Full commit SHA
	ShortSHA       string // Short SHA for display
	Message        string // Commit message (first line only)
	Author         string // Author name
	PRNumber       int64  // Associated PR/MR number (0 if none)
	QETestingLabel string // QE testing label status: "qe-tested", "needs-qe-testing", or empty
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

// Repository represents basic repository information
type Repository struct {
	Owner         string
	Name          string
	DefaultBranch string
	URL           string
}

// Documentation represents repository documentation
type Documentation struct {
	MainDocContent        string
	MainDocFile           string
	AdditionalDocsContent map[string]string // Successfully fetched linked docs: display name -> content
	AdditionalDocsOrder   []string          // Order of successfully fetched docs
	FailedAdditionalDocs  map[string]string // Failed linked docs: display name -> error message
	Repository            Repository
}

// UserGuidance represents a complete user guidance with metadata for reporting
type UserGuidance struct {
	Content      string    // The actual guidance content
	Author       string    // Platform username who posted it
	Date         time.Time // When it was posted
	CommentURL   string    // Direct link to the comment
	IsAuthorized bool      // Whether the author had permission to post
}
