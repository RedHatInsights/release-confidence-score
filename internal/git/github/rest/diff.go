package rest

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/go-github/v80/github"
	"golang.org/x/sync/errgroup"
	ghshared "release-confidence-score/internal/git/github/shared"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// fetchDiff fetches comparison data from GitHub REST API and augments commits with PR metadata
func fetchDiff(ctx context.Context, client *github.Client, owner, repo, base, head, diffURL string, cache *prCache) (*types.Comparison, error) {
	slog.Debug("Starting comparison fetch and commit augmentation", "owner", owner, "repo", repo, "base", base, "head", head)

	ghComparison, allCommits, err := ghshared.FetchComparisonWithPagination(ctx, client, owner, repo, base, head)
	if err != nil {
		return nil, err
	}

	slog.Debug("Fetched GitHub comparison", "commits", len(allCommits), "files", len(ghComparison.Files))

	comparison := &types.Comparison{
		RepoURL: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		DiffURL: diffURL,
		Commits: make([]types.Commit, len(allCommits)),
		Files:   ghshared.ConvertFiles(ghComparison.Files),
		Stats:   ghshared.CalculateStats(ghComparison.Files),
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i, commit := range allCommits {
		g.Go(func() error {
			comparison.Commits[i] = buildCommitEntry(gCtx, client, commit, owner, repo, cache)
			return nil
		})
	}
	g.Wait()

	slog.Debug("Commit augmentation complete", "commit_entries", len(comparison.Commits))

	return comparison, nil
}

// buildCommitEntry creates a commit entry from a GitHub commit with PR augmentation
func buildCommitEntry(ctx context.Context, client *github.Client, commit *github.RepositoryCommit, owner, repo string, cache *prCache) types.Commit {
	entry := ghshared.BuildBasicCommitEntry(commit)

	prNumber, err := getPRForCommit(ctx, client, owner, repo, entry.SHA)
	if err != nil {
		slog.Warn("Failed to find PR for commit", "commit", entry.ShortSHA, "error", err)
		return entry
	}

	if prNumber == 0 {
		slog.Debug("No PR found for commit", "commit", entry.ShortSHA)
		return entry
	}

	slog.Debug("Found PR for commit", "commit", entry.ShortSHA, "pr", prNumber)
	entry.PRNumber = int64(prNumber)

	pr, err := cache.getOrFetchPR(ctx, client, owner, repo, prNumber)
	if err != nil {
		slog.Warn("Failed to get PR object", "pr", prNumber, "error", err)
		return entry
	}

	entry.QETestingLabel = extractQELabel(pr)

	slog.Debug("Augmented commit", "commit", entry.ShortSHA, "pr", prNumber, "qe_label", entry.QETestingLabel)

	return entry
}

func getPRForCommit(ctx context.Context, client *github.Client, owner, repo, commitSHA string) (int, error) {
	prs, resp, err := client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, commitSHA, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to find PRs for commit %s: %w", commitSHA[:8], err)
	}

	slog.Debug("GitHub API response", "commit", commitSHA[:8], "found_prs", len(prs), "rate_limit_remaining", resp.Rate.Remaining)

	for _, pr := range prs {
		if !pr.GetMergedAt().IsZero() {
			return pr.GetNumber(), nil
		}
	}

	return 0, nil
}

// extractQELabel extracts the QE testing label from a PR
func extractQELabel(pr *github.PullRequest) string {
	if pr == nil {
		return ""
	}
	labelNames := make([]string, len(pr.Labels))
	for i, label := range pr.Labels {
		labelNames[i] = label.GetName()
	}
	return shared.ExtractQELabel(labelNames)
}

// prCache caches PR objects to avoid duplicate API calls.
type prCache struct {
	mu  sync.RWMutex
	prs map[int]*github.PullRequest
}

func newPRCache() *prCache {
	return &prCache{prs: make(map[int]*github.PullRequest)}
}

func (c *prCache) getOrFetchPR(ctx context.Context, client *github.Client, owner, repo string, prNumber int) (*github.PullRequest, error) {
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
