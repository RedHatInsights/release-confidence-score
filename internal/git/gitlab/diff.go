package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// fetchDiff fetches comparison data from GitLab and enriches commits with MR metadata and QE labels
// Returns a complete Comparison with enriched commits, files, and stats
// The cache parameter allows sharing cached MR objects across multiple operations
func fetchDiff(ctx context.Context, client *gitlab.Client, host, projectPath, base, head, diffURL string, cache *mrCache) (*types.Comparison, error) {
	slog.Debug("Starting comparison fetch and enrichment", "project", projectPath, "base", base, "head", head)

	// URL-encode project path for API calls
	encodedPath := url.PathEscape(projectPath)

	// Fetch comparison
	compareOpts := &gitlab.CompareOptions{
		From:     &base,
		To:       &head,
		Straight: gitlab.Ptr(false), // Use three-dot comparison (like GitHub)
	}
	compare, _, err := client.Repositories.Compare(encodedPath, compareOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comparison: %w", err)
	}

	slog.Debug("GitLab comparison fetched", "commits", len(compare.Commits), "diffs", len(compare.Diffs))

	// Convert diffs to files (calculates per-file stats once)
	files := convertDiffs(compare.Diffs)

	// Initialize comparison with files and stats from GitLab
	comparison := &types.Comparison{
		RepoURL: fmt.Sprintf("https://%s/%s", host, projectPath),
		DiffURL: diffURL,
		Commits: make([]types.Commit, 0, len(compare.Commits)),
		Files:   files,
		Stats:   calculateStats(files),
	}

	// Process each commit for enrichment (MR number, QE labels)
	for _, commit := range compare.Commits {
		commitEntry := buildCommitEntry(ctx, commit, client, encodedPath, cache)
		if commitEntry != nil {
			comparison.Commits = append(comparison.Commits, *commitEntry)
		}
	}

	slog.Debug("Commit enrichment complete", "commit_entries", len(comparison.Commits))

	return comparison, nil
}

// buildCommitEntry creates a commit entry from a GitLab commit with MR enrichment
func buildCommitEntry(ctx context.Context, commit *gitlab.Commit, client *gitlab.Client, projectPath string, cache *mrCache) *types.Commit {
	if commit == nil || commit.ID == "" {
		return nil
	}

	entry := &types.Commit{
		SHA:      commit.ID,
		ShortSHA: commit.ShortID,
		Message:  "No message",
		Author:   "Unknown",
	}

	// Extract commit message (first line only)
	if commit.Message != "" {
		entry.Message = strings.TrimSpace(strings.SplitN(commit.Message, "\n", 2)[0])
	}

	// Extract author name
	if commit.AuthorName != "" {
		entry.Author = commit.AuthorName
	}

	// Find MR for this commit
	mrIID, err := getMRForCommit(ctx, client, projectPath, entry.SHA)
	if err != nil {
		slog.Warn("Failed to find MR for commit", "commit", entry.ShortSHA, "error", err)
		return entry
	}

	if mrIID == 0 {
		slog.Debug("No MR found for commit", "commit", entry.ShortSHA)
		return entry
	}

	slog.Debug("Found MR for commit", "commit", entry.ShortSHA, "mr", mrIID)
	entry.PRNumber = mrIID

	// Get MR object (cached)
	mr, err := cache.getOrFetchMR(ctx, client, projectPath, mrIID)
	if err != nil {
		slog.Warn("Failed to get MR object", "mr", mrIID, "error", err)
		return entry
	}

	// Extract QE testing label
	entry.QETestingLabel = extractQELabel(mr)

	slog.Debug("Enriched commit", "commit", entry.ShortSHA, "mr", mrIID, "qe_label", entry.QETestingLabel)

	return entry
}

func getMRForCommit(ctx context.Context, client *gitlab.Client, projectPath, commitSHA string) (int64, error) {
	mrs, _, err := client.Commits.ListMergeRequestsByCommit(projectPath, commitSHA, gitlab.WithContext(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to get MRs for commit %s: %w", commitSHA[:8], err)
	}

	slog.Debug("GitLab API response", "commit", commitSHA[:8], "found_mrs", len(mrs))

	if len(mrs) == 0 {
		return 0, nil
	}

	// Use first merged MR, or first MR if none are merged
	for _, mr := range mrs {
		if mr.State == "merged" {
			return mr.IID, nil
		}
	}

	return mrs[0].IID, nil
}

// extractQELabel extracts the QE testing label from a GitLab MR
func extractQELabel(mr *gitlab.MergeRequest) string {
	if mr == nil {
		return ""
	}
	return shared.ExtractQELabel(mr.Labels)
}

// convertDiffs converts GitLab Diffs to platform-agnostic FileChanges
func convertDiffs(diffs []*gitlab.Diff) []types.FileChange {
	if diffs == nil {
		return []types.FileChange{}
	}

	result := make([]types.FileChange, 0, len(diffs))
	for _, diff := range diffs {
		result = append(result, convertDiff(diff))
	}
	return result
}

// convertDiff converts a GitLab Diff to platform-agnostic FileChange
func convertDiff(diff *gitlab.Diff) types.FileChange {
	if diff == nil {
		return types.FileChange{}
	}

	fileChange := types.FileChange{
		Filename:         diff.NewPath,
		Patch:            diff.Diff,
		PreviousFilename: diff.OldPath,
	}

	// Determine status
	if diff.NewFile {
		fileChange.Status = "added"
	} else if diff.DeletedFile {
		fileChange.Status = "removed"
	} else if diff.RenamedFile {
		fileChange.Status = "renamed"
	} else {
		fileChange.Status = "modified"
	}

	// Calculate additions/deletions from patch
	// GitLab doesn't provide these directly, so we parse the diff
	additions, deletions := parsePatchStats(diff.Diff)
	fileChange.Additions = additions
	fileChange.Deletions = deletions
	fileChange.Changes = additions + deletions

	return fileChange
}

// calculateStats calculates comparison statistics from converted files
func calculateStats(files []types.FileChange) types.ComparisonStats {
	stats := types.ComparisonStats{
		TotalFiles: len(files),
	}

	for _, file := range files {
		stats.TotalAdditions += file.Additions
		stats.TotalDeletions += file.Deletions
	}

	stats.TotalChanges = stats.TotalAdditions + stats.TotalDeletions
	return stats
}

// parsePatchStats counts additions and deletions from a unified diff patch
func parsePatchStats(patch string) (additions, deletions int) {
	if patch == "" {
		return 0, 0
	}

	for _, line := range strings.Split(patch, "\n") {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '+':
			if len(line) > 1 && line[1] != '+' { // Skip "+++ b/file" headers
				additions++
			}
		case '-':
			if len(line) > 1 && line[1] != '-' { // Skip "--- a/file" headers
				deletions++
			}
		}
	}

	return additions, deletions
}
