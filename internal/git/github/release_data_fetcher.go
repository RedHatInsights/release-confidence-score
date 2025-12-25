package github

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	githubapi "github.com/google/go-github/v80/github"
	"golang.org/x/sync/errgroup"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// githubCompareRegex matches GitHub compare URLs and extracts components
// Refs can be commit SHAs, tags (v1.0.0), or branches (main, feature/foo)
var githubCompareRegex = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/compare/(.+?)\.\.\.([^?#]+)$`)

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
// Returns: comparison data (with augmented commits, files, stats), user guidance list, documentation, error
// Documentation fetching runs in parallel with diff+guidance for better performance
func (f *Fetcher) FetchReleaseData(ctx context.Context, compareURL string) (*types.Comparison, []types.UserGuidance, *types.Documentation, error) {
	slog.Debug("Fetching GitHub release data", "url", compareURL)

	// Parse compare URL
	owner, repo, baseCommit, headCommit, err := parseCompareURL(compareURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse GitHub compare URL: %w", err)
	}

	slog.Debug("Parsed compare URL", "owner", owner, "repo", repo, "base", baseCommit, "head", headCommit)

	// Create shared cache to avoid duplicate API calls across operations
	cache := newPRCache()

	// Run documentation fetching in parallel with diff+guidance
	g, gCtx := errgroup.WithContext(ctx)

	var comparison *types.Comparison
	var userGuidance []types.UserGuidance
	var documentation *types.Documentation

	// Fetch diff and user guidance (sequential, as guidance depends on diff)
	g.Go(func() error {
		var err error
		comparison, err = fetchDiff(gCtx, f.client, owner, repo, baseCommit, headCommit, compareURL, cache)
		if err != nil {
			return fmt.Errorf("failed to fetch and enrich comparison: %w", err)
		}

		userGuidance, err = fetchUserGuidance(gCtx, f.client, owner, repo, comparison, cache)
		if err != nil {
			return fmt.Errorf("failed to fetch user guidance: %w", err)
		}
		return nil
	})

	// Fetch documentation (independent, runs in parallel)
	g.Go(func() error {
		docSource := newDocumentationSource(f.client, owner, repo)
		baseRepo := types.Repository{
			Owner: owner,
			Name:  repo,
			URL:   extractRepoURL(compareURL),
		}
		docFetcher := shared.NewDocumentationFetcher(docSource, baseRepo, f.config)

		var err error
		documentation, err = docFetcher.FetchAllDocs(gCtx)
		if err != nil {
			return fmt.Errorf("failed to fetch documentation: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, nil, nil, err
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
// Thread-safe for concurrent access during parallel commit enrichment.
type prCache struct {
	mu  sync.RWMutex
	prs map[int]*githubapi.PullRequest
}

func newPRCache() *prCache {
	return &prCache{prs: make(map[int]*githubapi.PullRequest)}
}

func (c *prCache) getOrFetchPR(ctx context.Context, client *githubapi.Client, owner, repo string, prNumber int) (*githubapi.PullRequest, error) {
	if prNumber == 0 {
		return nil, nil
	}

	c.mu.RLock()
	pr, exists := c.prs[prNumber]
	c.mu.RUnlock()
	if exists {
		slog.Debug("Using cached PR object", "pr", prNumber)
		return pr, nil
	}

	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "rate_limit_remaining", resp.Rate.Remaining)
	c.mu.Lock()
	c.prs[prNumber] = pr
	c.mu.Unlock()
	return pr, nil
}
