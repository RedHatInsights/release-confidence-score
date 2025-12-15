package types

import (
	"context"
)

// GitProvider represents a git hosting platform (GitHub, GitLab, etc.)
type GitProvider interface {
	// IsCompareURL checks if a URL is a valid compare URL for this platform
	IsCompareURL(url string) bool

	// FetchReleaseData fetches all release data for a compare URL including comparison data
	// (commits with metadata, files, stats), user guidance, and documentation
	// Returns: comparison data, user guidance list, documentation, error
	FetchReleaseData(compareURL string) (*Comparison, []UserGuidance, *Documentation, error)

	// Name returns the platform name (e.g., "GitHub", "GitLab")
	Name() string
}

// DocumentationSource defines the interface for fetching documentation from a repository
type DocumentationSource interface {
	// GetDefaultBranch returns the default branch name for the repository
	GetDefaultBranch(ctx context.Context) (string, error)

	// FetchFileContent fetches the content of a file from the repository
	FetchFileContent(ctx context.Context, path, ref string) (string, error)
}
