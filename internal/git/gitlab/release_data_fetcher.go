package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// gitlabCompareRegex matches GitLab compare URLs and extracts components
// Format: https://gitlab.com/owner/repo/-/compare/base...head
// or: https://gitlab.com/group/subgroup/repo/-/compare/base...head
// Refs can be commit SHAs, tags (v1.0.0), or branches (main, feature/foo)
var gitlabCompareRegex = regexp.MustCompile(`^https?://([^/]+)/(.+)/-/compare/(.+?)\.\.\.([^?#]+)`)

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
	return gitlabCompareRegex.MatchString(url)
}

// FetchReleaseData fetches all release data for a GitLab compare URL
// Returns: comparison data (with enriched commits, files, stats), user guidance list, documentation, error
func (f *Fetcher) FetchReleaseData(compareURL string) (*types.Comparison, []types.UserGuidance, *types.Documentation, error) {
	slog.Debug("Fetching GitLab release data", "url", compareURL)

	// Parse compare URL
	host, projectPath, baseCommit, headCommit, err := parseCompareURL(compareURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse GitLab compare URL: %w", err)
	}

	slog.Debug("Parsed compare URL", "host", host, "project", projectPath, "base", baseCommit, "head", headCommit)

	// URL-encode project path for API calls
	encodedPath := urlEncodeProjectPath(projectPath)

	// Create shared cache to avoid duplicate API calls across operations
	cache := newMRCache()

	// Fetch comparison and enrich commits with MR metadata and QE labels
	comparison, err := fetchDiff(context.Background(), f.client, host, projectPath, baseCommit, headCommit, compareURL, cache)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch and enrich comparison: %w", err)
	}

	// Extract user guidance from MRs in the comparison (reuses cached MR objects)
	userGuidance, err := fetchUserGuidance(context.Background(), f.client, encodedPath, comparison, cache)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch user guidance: %w", err)
	}

	// Fetch documentation
	docSource := newDocumentationSource(f.client, host, projectPath)
	owner, name := splitProjectPath(projectPath)
	baseRepo := types.Repository{
		Owner: owner,
		Name:  name,
		URL:   comparison.RepoURL,
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

// splitProjectPath splits GitLab project path into owner and name
// For "group/repo" returns ("group", "repo")
// For "group/subgroup/repo" returns ("group/subgroup", "repo")
// For "repo" returns ("", "repo")
func splitProjectPath(projectPath string) (owner, name string) {
	lastSlash := strings.LastIndex(projectPath, "/")
	if lastSlash == -1 {
		return "", projectPath
	}
	return projectPath[:lastSlash], projectPath[lastSlash+1:]
}

func urlEncodeProjectPath(projectPath string) string {
	return url.PathEscape(projectPath)
}

// mrCache caches MR objects to avoid duplicate API calls within a single CLI execution.
// Multiple commits often belong to the same MR, so caching avoids re-fetching.
type mrCache struct {
	mergeRequests map[int64]*gitlabapi.MergeRequest
}

func newMRCache() *mrCache {
	return &mrCache{mergeRequests: make(map[int64]*gitlabapi.MergeRequest)}
}

func (c *mrCache) getOrFetchMR(ctx context.Context, client *gitlabapi.Client, projectPath string, mrIID int64) (*gitlabapi.MergeRequest, error) {
	if mrIID == 0 {
		return nil, nil
	}

	if mr, exists := c.mergeRequests[mrIID]; exists {
		slog.Debug("Using cached MR object", "mr", mrIID)
		return mr, nil
	}

	mr, _, err := client.MergeRequests.GetMergeRequest(projectPath, mrIID, &gitlabapi.GetMergeRequestsOptions{}, gitlabapi.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get MR !%d: %w", mrIID, err)
	}

	slog.Debug("GitLab API response", "mr", mrIID)
	c.mergeRequests[mrIID] = mr

	return mr, nil
}
