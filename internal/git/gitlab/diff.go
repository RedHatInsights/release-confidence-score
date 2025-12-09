package gitlab

import (
	"fmt"
	"log/slog"
	"strings"

	"gitlab.com/gitlab-org/api/client-go"
	"release-confidence-score/internal/git/types"
)

// fetchDiff fetches comparison data from GitLab and enriches commits with MR metadata and QE labels
// Returns a complete Comparison with enriched commits, files, and stats
func fetchDiff(client *gitlab.Client, projectPath, baseRef, headRef, compareURL string) (*types.Comparison, error) {
	slog.Debug("Starting comparison fetch and enrichment", "project", projectPath, "base", baseRef, "head", headRef)

	// Fetch comparison
	compare, _, err := client.Repositories.Compare(projectPath, &gitlab.CompareOptions{
		From:     &baseRef,
		To:       &headRef,
		Straight: gitlab.Ptr(false), // Use three-dot comparison (like GitHub)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comparison: %w", err)
	}

	slog.Debug("GitLab comparison fetched", "commits", len(compare.Commits), "diffs", len(compare.Diffs))

	// Build repo URL
	repoURL := extractRepoURL(compareURL)

	// Create internal cache to avoid duplicate API calls
	cache := newMRCache()

	// Initialize comparison with files and stats from GitLab
	comparison := &types.Comparison{
		RepoURL: repoURL,
		DiffURL: compareURL,
		Commits: make([]types.Commit, 0, len(compare.Commits)),
		Files:   convertDiffs(compare.Diffs),
		Stats:   calculateStatsFromDiffs(compare.Diffs),
	}

	// Process each commit for enrichment (MR number, QE labels)
	for _, commit := range compare.Commits {
		if commit == nil || commit.ID == "" {
			continue
		}

		// Build commit entry with MR enrichment
		commitEntry := buildCommitEntry(commit, client, projectPath, repoURL, cache)
		if commitEntry != nil {
			comparison.Commits = append(comparison.Commits, *commitEntry)
		}
	}

	slog.Debug("Commit enrichment complete", "commit_entries", len(comparison.Commits))

	return comparison, nil
}

// buildCommitEntry creates a commit entry from a GitLab commit with MR enrichment
func buildCommitEntry(commit *gitlab.Commit, client *gitlab.Client, projectPath, repoURL string, cache *mrCache) *types.Commit {
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
		lines := strings.Split(commit.Message, "\n")
		if len(lines) > 0 {
			entry.Message = strings.TrimSpace(lines[0])
		}
	}

	// Extract author name
	if commit.AuthorName != "" {
		entry.Author = commit.AuthorName
	}

	// Find MR for this commit (cached)
	mrIID, err := cache.getOrFetchMRForCommit(client, projectPath, entry.SHA)
	if err != nil {
		slog.Warn("Failed to find MR for commit", "commit", entry.ShortSHA, "error", err)
		return entry
	}

	if mrIID == 0 {
		slog.Debug("No MR found for commit", "commit", entry.ShortSHA)
		return entry
	}

	slog.Debug("Found MR for commit", "commit", entry.ShortSHA, "mr_iid", mrIID)
	entry.PRNumber = mrIID

	// Get MR object (cached)
	mr, err := cache.getOrFetchMR(client, projectPath, mrIID)
	if err != nil {
		slog.Warn("Failed to get MR object", "mr_iid", mrIID, "error", err)
		return entry
	}

	// Extract QE testing label
	qeLabel := extractQELabel(mr)
	entry.QETestingLabel = qeLabel

	slog.Debug("Enriched commit", "commit", entry.ShortSHA, "mr", mrIID, "qe_label", qeLabel)

	return entry
}

// extractQELabel extracts the QE testing label from a GitLab MR
// Returns "qe-tested", "needs-qe-testing", or empty string
func extractQELabel(mr *gitlab.MergeRequest) string {
	if mr == nil || mr.Labels == nil {
		return ""
	}

	hasQeTested := false
	hasNeedsQETesting := false

	for _, label := range mr.Labels {
		labelLower := strings.ToLower(label)
		if labelLower == "rcs/qe-tested" {
			hasQeTested = true
		} else if labelLower == "rcs/needs-qe-testing" {
			hasNeedsQETesting = true
		}
	}

	// Priority logic: qe-tested takes precedence over needs-qe-testing
	if hasQeTested {
		return "qe-tested"
	} else if hasNeedsQETesting {
		return "needs-qe-testing"
	}
	return ""
}

// mrCache caches GitLab API responses to avoid duplicate calls
// Internal to enrichment - not exposed outside this file
type mrCache struct {
	commitToMR    map[string]int                  // commit SHA -> MR IID
	mergeRequests map[string]*gitlab.MergeRequest // "project/mriid" -> MR object
	mrNotes       map[string][]*gitlab.Note       // "project/mriid" -> MR notes
	mrApprovers   map[string][]string             // "project/mriid" -> list of approver usernames
}

func newMRCache() *mrCache {
	return &mrCache{
		commitToMR:    make(map[string]int),
		mergeRequests: make(map[string]*gitlab.MergeRequest),
		mrNotes:       make(map[string][]*gitlab.Note),
		mrApprovers:   make(map[string][]string),
	}
}

func cacheKey(projectPath string, mrIID int) string {
	return fmt.Sprintf("%s/%d", projectPath, mrIID)
}

// getCommitMRIID gets cached MR IID for a commit (doesn't fetch if not in cache)
func (c *mrCache) getCommitMRIID(commitSHA string) int {
	return c.commitToMR[commitSHA]
}

// getMR gets cached MR object (doesn't fetch if not in cache)
func (c *mrCache) getMR(mrIID int) *gitlab.MergeRequest {
	for _, mr := range c.mergeRequests {
		if mr.IID == mrIID {
			return mr
		}
	}
	return nil
}

func (c *mrCache) getOrFetchMRForCommit(client *gitlab.Client, projectPath, commitSHA string) (int, error) {
	// Check cache first
	if mrIID, exists := c.commitToMR[commitSHA]; exists {
		slog.Debug("Using cached commit→MR mapping", "commit", commitSHA[:8], "mr_iid", mrIID)
		return mrIID, nil
	}

	// Cache miss - fetch from API
	mrs, _, err := client.Commits.ListMergeRequestsByCommit(projectPath, commitSHA)
	if err != nil {
		return 0, fmt.Errorf("failed to get MRs for commit %s: %w", commitSHA[:8], err)
	}

	slog.Debug("GitLab API response", "commit", commitSHA[:8], "found_mrs", len(mrs))

	// If no MRs found, cache 0 to avoid re-fetching
	if len(mrs) == 0 {
		c.commitToMR[commitSHA] = 0
		return 0, nil
	}

	// Use the first merged MR (or first MR if none are merged)
	var selectedMR *gitlab.BasicMergeRequest
	for _, mr := range mrs {
		if mr.State == "merged" {
			selectedMR = mr
			break
		}
	}
	if selectedMR == nil {
		selectedMR = mrs[0]
	}

	// Cache the result
	c.commitToMR[commitSHA] = selectedMR.IID

	return selectedMR.IID, nil
}

func (c *mrCache) getOrFetchMR(client *gitlab.Client, projectPath string, mrIID int) (*gitlab.MergeRequest, error) {
	if mrIID == 0 {
		return nil, nil
	}

	key := cacheKey(projectPath, mrIID)

	// Check cache first
	if mr, exists := c.mergeRequests[key]; exists {
		slog.Debug("Using cached MR object", "mr_iid", mrIID)
		return mr, nil
	}

	// Cache miss - fetch from API
	mr, _, err := client.MergeRequests.GetMergeRequest(projectPath, mrIID, &gitlab.GetMergeRequestsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get MR !%d: %w", mrIID, err)
	}

	slog.Debug("GitLab API response", "mr_iid", mrIID)

	// Cache the result
	c.mergeRequests[key] = mr

	return mr, nil
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

// parsePatchStats counts additions and deletions from a unified diff patch
func parsePatchStats(patch string) (additions, deletions int) {
	if patch == "" {
		return 0, 0
	}

	lines := splitLines(patch)
	for _, line := range lines {
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

// splitLines splits a string by newlines
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0, 100)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// calculateStatsFromDiffs calculates comparison statistics from GitLab diffs
func calculateStatsFromDiffs(diffs []*gitlab.Diff) types.ComparisonStats {
	stats := types.ComparisonStats{
		TotalFiles: len(diffs),
	}

	for _, diff := range diffs {
		if diff == nil {
			continue
		}
		additions, deletions := parsePatchStats(diff.Diff)
		stats.TotalAdditions += additions
		stats.TotalDeletions += deletions
	}

	stats.TotalChanges = stats.TotalAdditions + stats.TotalDeletions
	return stats
}
