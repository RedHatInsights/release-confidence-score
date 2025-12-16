package github

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"

	"github.com/google/go-github/v80/github"
)

// fetchDiff fetches comparison data from GitHub and enriches commits with PR metadata and QE labels
// Returns a complete Comparison with enriched commits, files, and stats
// The cache parameter allows sharing cached PR objects across multiple operations
func fetchDiff(ctx context.Context, client *github.Client, owner, repo, base, head, diffURL string, cache *prCache) (*types.Comparison, error) {
	slog.Debug("Starting comparison fetch and enrichment", "owner", owner, "repo", repo, "base", base, "head", head)

	// Fetch comparison data with all commits (handles pagination)
	ghComparison, allCommits, err := fetchComparisonWithPagination(ctx, client, owner, repo, base, head)
	if err != nil {
		return nil, err
	}

	slog.Debug("Fetched GitHub comparison", "commits", len(allCommits), "files", len(ghComparison.Files))

	// Initialize comparison with files and stats from GitHub
	comparison := &types.Comparison{
		RepoURL: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		DiffURL: diffURL,
		Commits: make([]types.Commit, 0, len(allCommits)),
		Files:   convertFiles(ghComparison.Files),
		Stats:   calculateStats(ghComparison.Files),
	}

	// Process each commit for enrichment (PR number, QE labels)
	for _, commit := range allCommits {
		commitEntry := buildCommitEntry(ctx, commit, client, owner, repo, cache)
		if commitEntry != nil {
			comparison.Commits = append(comparison.Commits, *commitEntry)
		}
	}

	slog.Debug("Commit enrichment complete", "commit_entries", len(comparison.Commits))

	return comparison, nil
}

// buildCommitEntry creates a commit entry from a GitHub commit with PR enrichment
func buildCommitEntry(ctx context.Context, commit *github.RepositoryCommit, client *github.Client, owner, repo string, cache *prCache) *types.Commit {
	if commit == nil || commit.SHA == nil || *commit.SHA == "" {
		return nil
	}

	entry := &types.Commit{
		SHA:      *commit.SHA,
		ShortSHA: (*commit.SHA)[:8],
		Message:  "No message",
		Author:   "Unknown",
	}

	// Extract commit message (first line only)
	if msg := commit.GetCommit().GetMessage(); msg != "" {
		entry.Message = strings.TrimSpace(strings.SplitN(msg, "\n", 2)[0])
	}

	// Extract author name
	if name := commit.GetCommit().GetAuthor().GetName(); name != "" {
		entry.Author = name
	}

	// Find PR for this commit
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

	// Get PR object (cached)
	pr, err := cache.getOrFetchPR(ctx, client, owner, repo, prNumber)
	if err != nil {
		slog.Warn("Failed to get PR object", "pr", prNumber, "error", err)
		return entry
	}

	// Extract QE testing label
	entry.QETestingLabel = extractQELabel(pr)

	slog.Debug("Enriched commit", "commit", entry.ShortSHA, "pr", prNumber, "qe_label", entry.QETestingLabel)

	return entry
}

func getPRForCommit(ctx context.Context, client *github.Client, owner, repo, commitSHA string) (int, error) {
	prs, resp, err := client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, commitSHA, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to find PRs for commit %s: %w", commitSHA[:8], err)
	}

	slog.Debug("GitHub API response", "commit", commitSHA[:8], "found_prs", len(prs), "rate_limit_remaining", resp.Rate.Remaining)

	// Find first merged PR
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

// fetchComparisonWithPagination fetches comparison data with full commit pagination
// GitHub API limits commits per page, so we need to paginate to get all commits
func fetchComparisonWithPagination(ctx context.Context, client *github.Client, owner, repo, base, head string) (*github.CommitsComparison, []*github.RepositoryCommit, error) {
	var allCommits []*github.RepositoryCommit
	var comparisonData *github.CommitsComparison
	opts := &github.ListOptions{Page: 1, PerPage: 100}

	for {
		comparison, resp, err := client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch comparison from GitHub (page %d, owner=%s, repo=%s, base=%s, head=%s): %w",
				opts.Page, owner, repo, base, head, err)
		}

		// Store comparison data from first page
		if opts.Page == 1 {
			comparisonData = comparison
		}

		// Collect commits from this page
		if comparison.Commits != nil {
			allCommits = append(allCommits, comparison.Commits...)
		}

		// Check if we have more pages
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return comparisonData, allCommits, nil
}

// convertFiles converts GitHub CommitFiles to platform-agnostic FileChanges
func convertFiles(files []*github.CommitFile) []types.FileChange {
	if files == nil {
		return []types.FileChange{}
	}

	result := make([]types.FileChange, 0, len(files))
	for _, file := range files {
		result = append(result, convertFile(file))
	}
	return result
}

// convertFile converts a GitHub CommitFile to platform-agnostic FileChange
func convertFile(file *github.CommitFile) types.FileChange {
	if file == nil {
		return types.FileChange{}
	}
	return types.FileChange{
		Filename:         file.GetFilename(),
		Status:           file.GetStatus(),
		Additions:        file.GetAdditions(),
		Deletions:        file.GetDeletions(),
		Changes:          file.GetChanges(),
		Patch:            file.GetPatch(),
		PreviousFilename: file.GetPreviousFilename(),
	}
}

// calculateStats calculates comparison statistics from GitHub files
func calculateStats(files []*github.CommitFile) types.ComparisonStats {
	stats := types.ComparisonStats{
		TotalFiles: len(files),
	}

	for _, file := range files {
		if file == nil {
			continue
		}
		stats.TotalAdditions += file.GetAdditions()
		stats.TotalDeletions += file.GetDeletions()
	}

	stats.TotalChanges = stats.TotalAdditions + stats.TotalDeletions
	return stats
}
