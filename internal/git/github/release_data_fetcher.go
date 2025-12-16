package github

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	githubapi "github.com/google/go-github/v80/github"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// githubCompareRegex matches GitHub compare URLs and extracts components
var githubCompareRegex = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/compare/([a-f0-9]+)\.\.\.([a-f0-9]+)$`)

// Fetcher implements the GitProvider interface for GitHub
type Fetcher struct {
	client *githubapi.Client
	config *config.Config
}

// NewFetcher creates a new GitHub data fetcher
func NewFetcher(client *githubapi.Client, cfg *config.Config) *Fetcher {
	return &Fetcher{
		client: client,
		config: cfg,
	}
}

// Name returns the platform name
func (f *Fetcher) Name() string {
	return "GitHub"
}

// IsCompareURL checks if a URL is a valid GitHub compare URL
func (f *Fetcher) IsCompareURL(url string) bool {
	return githubCompareRegex.MatchString(url)
}

// FetchReleaseData fetches all release data for a GitHub compare URL
// Returns: comparison data (with enriched commits, files, stats), user guidance list, documentation, error
func (f *Fetcher) FetchReleaseData(compareURL string) (*types.Comparison, []types.UserGuidance, *types.Documentation, error) {
	slog.Debug("Fetching GitHub release data", "url", compareURL)

	// Parse compare URL
	owner, repo, baseCommit, headCommit, err := parseCompareURL(compareURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse GitHub compare URL: %w", err)
	}

	slog.Debug("Parsed compare URL", "owner", owner, "repo", repo, "base", baseCommit, "head", headCommit)

	// Create shared cache to avoid duplicate API calls across operations
	cache := newPRCache()

	// Fetch comparison and enrich commits with PR metadata and QE labels
	comparison, err := fetchDiff(context.Background(), f.client, owner, repo, baseCommit, headCommit, compareURL, cache)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch and enrich comparison: %w", err)
	}

	// Extract user guidance from PRs in the comparison (reuses cached PR objects)
	userGuidance, err := fetchUserGuidance(context.Background(), f.client, owner, repo, comparison, cache)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch user guidance: %w", err)
	}

	// Fetch documentation
	docSource := newDocumentationSource(f.client, owner, repo)
	baseRepo := types.Repository{
		Owner: owner,
		Name:  repo,
		URL:   extractRepoURL(compareURL),
	}
	docFetcher := shared.NewDocumentationFetcher(docSource, baseRepo, f.config)
	documentation, err := docFetcher.FetchAllDocs(context.Background())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch documentation: %w", err)
	}

	slog.Debug("Release data fetched successfully",
		"commit_entries", len(comparison.Commits),
		"user_guidance_items", len(userGuidance),
		"files", len(comparison.Files),
		"has_documentation", documentation != nil)

	return comparison, userGuidance, documentation, nil
}

// parseCompareURL extracts owner, repo, baseCommit, and headCommit from GitHub compare URL
// Returns: owner, repo, base commit SHA, head commit SHA, error
func parseCompareURL(compareURL string) (owner, repo, baseCommit, headCommit string, err error) {
	// Parse: https://github.com/owner/repo/compare/sha1...sha2
	matches := githubCompareRegex.FindStringSubmatch(compareURL)
	if len(matches) != 5 {
		return "", "", "", "", fmt.Errorf("invalid GitHub compare URL format: %s", compareURL)
	}

	return matches[1], matches[2], matches[3], matches[4], nil
}

// extractRepoURL extracts the repository URL from a compare URL
// e.g., "https://github.com/owner/repo/compare/..." -> "https://github.com/owner/repo"
func extractRepoURL(compareURL string) string {
	// Find "/compare/" and take everything before it
	if idx := strings.Index(compareURL, "/compare/"); idx != -1 {
		return compareURL[:idx]
	}
	return compareURL
}

// prCache caches PR objects to avoid duplicate API calls within a single CLI execution.
// Multiple commits often belong to the same PR, so caching avoids re-fetching.
type prCache struct {
	prs map[int]*githubapi.PullRequest
}

func newPRCache() *prCache {
	return &prCache{prs: make(map[int]*githubapi.PullRequest)}
}

func (c *prCache) getOrFetchPR(ctx context.Context, client *githubapi.Client, owner, repo string, prNumber int) (*githubapi.PullRequest, error) {
	if prNumber == 0 {
		return nil, nil
	}

	if pr, exists := c.prs[prNumber]; exists {
		slog.Debug("Using cached PR object", "pr", prNumber)
		return pr, nil
	}

	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "rate_limit_remaining", resp.Rate.Remaining)
	c.prs[prNumber] = pr
	return pr, nil
}
