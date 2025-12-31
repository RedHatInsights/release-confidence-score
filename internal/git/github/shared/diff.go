package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v80/github"
	"release-confidence-score/internal/git/types"
)

// NewRESTClient creates a new GitHub REST API client
func NewRESTClient(token string) *github.Client {
	return github.NewClient(nil).WithAuthToken(token)
}

// FetchComparisonWithPagination fetches comparison data with full commit pagination
func FetchComparisonWithPagination(ctx context.Context, client *github.Client, owner, repo, base, head string) (*github.CommitsComparison, []*github.RepositoryCommit, error) {
	var allCommits []*github.RepositoryCommit
	var comparisonData *github.CommitsComparison
	opts := &github.ListOptions{Page: 1, PerPage: 100}

	for {
		comparison, resp, err := client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch comparison from GitHub (page %d, owner=%s, repo=%s, base=%s, head=%s): %w",
				opts.Page, owner, repo, base, head, err)
		}

		if opts.Page == 1 {
			comparisonData = comparison
		}

		if comparison.Commits != nil {
			allCommits = append(allCommits, comparison.Commits...)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return comparisonData, allCommits, nil
}

// ConvertFiles converts GitHub CommitFiles to platform-agnostic FileChanges
func ConvertFiles(files []*github.CommitFile) []types.FileChange {
	if files == nil {
		return []types.FileChange{}
	}

	result := make([]types.FileChange, 0, len(files))
	for _, file := range files {
		result = append(result, ConvertFile(file))
	}
	return result
}

// ConvertFile converts a single GitHub CommitFile to platform-agnostic FileChange
func ConvertFile(file *github.CommitFile) types.FileChange {
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

// CalculateStats calculates comparison statistics from GitHub files
func CalculateStats(files []*github.CommitFile) types.ComparisonStats {
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

// BuildBasicCommitEntry creates a commit entry with basic info from a GitHub commit
func BuildBasicCommitEntry(commit *github.RepositoryCommit) types.Commit {
	entry := types.Commit{
		SHA:      commit.GetSHA(),
		ShortSHA: commit.GetSHA()[:8],
		Message:  "No message",
		Author:   "Unknown",
	}

	if msg := commit.GetCommit().GetMessage(); msg != "" {
		entry.Message = strings.TrimSpace(strings.SplitN(msg, "\n", 2)[0])
	}

	if name := commit.GetCommit().GetAuthor().GetName(); name != "" {
		entry.Author = name
	}

	return entry
}
