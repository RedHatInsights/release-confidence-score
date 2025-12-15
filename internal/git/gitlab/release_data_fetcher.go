package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/types"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// gitlabCompareRegex matches GitLab compare URLs and extracts components
// Format: https://gitlab.com/owner/repo/-/compare/base...head
// or: https://gitlab.com/group/subgroup/repo/-/compare/base...head
var gitlabCompareRegex = regexp.MustCompile(`^https?://([^/]+)/(.+)/-/compare/([^.]+)\.\.\.([^?#]+)`)

// Fetcher implements the GitProvider interface for GitLab
type Fetcher struct {
	client *gitlabapi.Client
	config *config.Config
}

// NewFetcher creates a new GitLab data fetcher
func NewFetcher(client *gitlabapi.Client, cfg *config.Config) *Fetcher {
	return &Fetcher{
		client: client,
		config: cfg,
	}
}

// Name returns the platform name
func (f *Fetcher) Name() string {
	return "GitLab"
}

// IsCompareURL checks if a URL is a valid GitLab compare URL
func (f *Fetcher) IsCompareURL(url string) bool {
	return IsGitLabCompareURL(url)
}

// FetchReleaseData fetches all release data for a GitLab compare URL
// Returns: comparison data (with enriched commits, files, stats), user guidance list, documentation, error
func (f *Fetcher) FetchReleaseData(compareURL string) (*types.Comparison, []types.UserGuidance, *types.Documentation, error) {
	slog.Debug("Fetching GitLab release data", "url", compareURL)

	// Parse compare URL
	host, projectPath, baseRef, headRef, err := parseCompareURL(compareURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse GitLab compare URL: %w", err)
	}

	slog.Debug("Parsed compare URL", "host", host, "project", projectPath, "base", baseRef, "head", headRef)

	// URL-encode project path for API calls
	encodedPath := urlEncodeProjectPath(projectPath)

	// Fetch documentation
	documentation, err := fetchDocumentation(f.client, host, projectPath, f.config)
	if err != nil {
		slog.Debug("Failed to fetch documentation (non-fatal)", "error", err)
		documentation = nil
	}

	// Fetch comparison and enrich commits with MR metadata and QE labels
	comparison, err := fetchDiff(f.client, encodedPath, baseRef, headRef, compareURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch and enrich comparison: %w", err)
	}

	// Extract user guidance from MRs in the comparison
	userGuidance, err := fetchUserGuidance(context.Background(), f.client, encodedPath, comparison)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch user guidance: %w", err)
	}

	slog.Debug("Release data fetched successfully",
		"commit_entries", len(comparison.Commits),
		"user_guidance_items", len(userGuidance),
		"files", len(comparison.Files),
		"has_documentation", documentation != nil)

	return comparison, userGuidance, documentation, nil
}

// IsGitLabCompareURL checks if a URL is a GitLab compare URL
func IsGitLabCompareURL(url string) bool {
	return gitlabCompareRegex.MatchString(url)
}

// parseCompareURL extracts host, project path, baseRef, and headRef from GitLab compare URL
// Returns: host, projectPath, baseRef, headRef, error
func parseCompareURL(compareURL string) (host, projectPath, baseRef, headRef string, err error) {
	// Parse: https://gitlab.com/owner/repo/-/compare/sha1...sha2
	// or: https://gitlab.com/group/subgroup/repo/-/compare/sha1...sha2
	matches := gitlabCompareRegex.FindStringSubmatch(compareURL)
	if len(matches) != 5 {
		return "", "", "", "", fmt.Errorf("invalid GitLab compare URL format: %s", compareURL)
	}

	return matches[1], matches[2], matches[3], matches[4], nil
}

// extractRepoURL extracts the repository URL from a compare URL
// e.g., "https://gitlab.com/owner/repo/-/compare/..." -> "https://gitlab.com/owner/repo"
func extractRepoURL(compareURL string) string {
	// GitLab uses "/-/compare/" format
	if idx := strings.Index(compareURL, "/-/compare/"); idx != -1 {
		return compareURL[:idx]
	}
	// Fallback to "/compare/" for other formats
	if idx := strings.Index(compareURL, "/compare/"); idx != -1 {
		return compareURL[:idx]
	}
	return compareURL
}

// urlEncodeProjectPath URL-encodes a GitLab project path for API calls
func urlEncodeProjectPath(projectPath string) string {
	return url.PathEscape(projectPath)
}
