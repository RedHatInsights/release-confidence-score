package gitlab

import (
	"context"
	"fmt"
	"log/slog"

	"gitlab.com/gitlab-org/api/client-go"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// fetchUserGuidance extracts user guidance from all MRs in the comparison
// The cache parameter allows reusing MR objects already fetched during diff enrichment
func fetchUserGuidance(ctx context.Context, client *gitlab.Client, projectPath string, comparison *types.Comparison, cache *mrCache) ([]types.UserGuidance, error) {
	if comparison == nil || len(comparison.Commits) == 0 {
		return []types.UserGuidance{}, nil
	}

	slog.Debug("Extracting user guidance from comparison", "commits", len(comparison.Commits))

	var allGuidance []types.UserGuidance

	// Track which MRs we've already processed to avoid duplicates
	processedMRs := make(map[int64]bool)

	// Extract guidance from each unique MR
	for _, commit := range comparison.Commits {
		if commit.PRNumber == 0 || processedMRs[commit.PRNumber] {
			continue
		}

		processedMRs[commit.PRNumber] = true

		// Get MR object (uses cache populated during diff enrichment)
		mr, err := cache.getOrFetchMR(ctx, client, projectPath, commit.PRNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch MR !%d for guidance extraction: %w", commit.PRNumber, err)
		}

		if mr == nil {
			continue
		}

		// Extract guidance from this MR
		slog.Debug("Extracting user guidance from MR", "mr", commit.PRNumber)
		guidance, err := extractUserGuidance(ctx, client, projectPath, comparison.RepoURL, mr)
		if err != nil {
			return nil, fmt.Errorf("failed to extract user guidance from MR !%d: %w", commit.PRNumber, err)
		}

		if len(guidance) > 0 {
			allGuidance = append(allGuidance, guidance...)
		}
	}

	slog.Debug("User guidance extraction complete", "items", len(allGuidance))
	return allGuidance, nil
}

// extractUserGuidance extracts all user guidance from a MR's notes
func extractUserGuidance(ctx context.Context, client *gitlab.Client, projectPath, repoURL string, mr *gitlab.MergeRequest) ([]types.UserGuidance, error) {
	if mr == nil {
		return nil, nil
	}

	mrIID := mr.IID
	var allGuidance []types.UserGuidance

	// Fetch all notes for this MR with pagination
	var allNotes []*gitlab.Note
	opts := &gitlab.ListMergeRequestNotesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	for {
		notes, resp, err := client.Notes.ListMergeRequestNotes(projectPath, mrIID, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get notes for MR !%d: %w", mrIID, err)
		}

		allNotes = append(allNotes, notes...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Fetch MR approvers for authorization check
	approvers, err := getMRApprovers(ctx, client, projectPath, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MR approvers for !%d: %w", mrIID, err)
	}

	// Process each note
	for _, note := range allNotes {
		if !isValidNote(note) {
			continue
		}

		mrAuthor := ""
		if mr.Author != nil {
			mrAuthor = mr.Author.Username
		}

		guidance := processNote(note, mrAuthor, approvers, repoURL, mrIID)
		if guidance != nil {
			allGuidance = append(allGuidance, *guidance)
		}
	}

	return allGuidance, nil
}

// processNote processes a single note and returns user guidance if found
func processNote(note *gitlab.Note, mrAuthor string, approvers []string, repoURL string, mrIID int64) *types.UserGuidance {
	guidanceContent, found := shared.ParseUserGuidance(note.Body)
	if !found {
		return nil
	}

	isAuthorized := isAuthorized(note.Author.Username, mrAuthor, approvers, mrIID)

	// Construct comment URL: {repoURL}/-/merge_requests/{mrIID}#note_{noteID}
	commentURL := fmt.Sprintf("%s/-/merge_requests/%d#note_%d", repoURL, mrIID, note.ID)

	slog.Debug("Found user guidance in MR note", "mr_iid", mrIID, "author", note.Author.Username, "authorized", isAuthorized)

	return &types.UserGuidance{
		Content:      guidanceContent,
		Author:       note.Author.Username,
		Date:         *note.CreatedAt,
		CommentURL:   commentURL,
		IsAuthorized: isAuthorized,
	}
}

// isValidNote checks if a note has all required fields
func isValidNote(note *gitlab.Note) bool {
	if note == nil || note.Body == "" {
		return false
	}
	if note.Author.Username == "" {
		return false
	}
	if note.CreatedAt == nil {
		return false
	}
	return true
}

// getMRApprovers returns the list of usernames who approved the MR
func getMRApprovers(ctx context.Context, client *gitlab.Client, projectPath string, mrIID int64) ([]string, error) {
	// Get MR approval state
	approvals, _, err := client.MergeRequestApprovals.GetConfiguration(projectPath, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MR approvals: %w", err)
	}

	var approvers []string
	if approvals.ApprovedBy != nil {
		for _, approver := range approvals.ApprovedBy {
			if approver.User != nil && approver.User.Username != "" {
				approvers = append(approvers, approver.User.Username)
			}
		}
	}

	return approvers, nil
}

// isAuthorized checks if a user is authorized to provide guidance
// Authorization criteria:
// 1. User is the MR author, OR
// 2. User is in the list of MR approvers
//
// Note: GitLab's approval system is permission-based, so we don't need to check
// additional authority levels - if they're in the approvers list, they're authorized.
func isAuthorized(username, mrAuthor string, approvers []string, mrIID int64) bool {
	// Check if user is MR author
	if username == mrAuthor {
		slog.Debug("User authorized as MR author", "user", username, "mr_iid", mrIID)
		return true
	}

	// Check if user is an approver
	for _, approver := range approvers {
		if username == approver {
			slog.Debug("User authorized as approver", "user", username, "mr_iid", mrIID)
			return true
		}
	}

	slog.Debug("User not authorized", "user", username, "mr_iid", mrIID)
	return false
}
