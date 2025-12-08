package gitlab

import (
	"fmt"
	"log/slog"

	"gitlab.com/gitlab-org/api/client-go"
	"release-confidence-score/internal/git/types"
)

// FetchUserGuidance extracts user guidance from all MRs in the comparison
func FetchUserGuidance(client *gitlab.Client, projectPath string, comparison *types.Comparison) ([]types.UserGuidance, error) {
	if comparison == nil || len(comparison.Commits) == 0 {
		return []types.UserGuidance{}, nil
	}

	slog.Debug("Extracting user guidance from comparison", "commits", len(comparison.Commits))

	// Create cache to avoid duplicate API calls
	cache := newMRCache()
	var allGuidance []types.UserGuidance

	// Track which MRs we've already processed to avoid duplicates
	processedMRs := make(map[int]bool)

	// Extract guidance from each unique MR
	for _, commit := range comparison.Commits {
		if commit.PRNumber == 0 || processedMRs[commit.PRNumber] {
			continue
		}

		processedMRs[commit.PRNumber] = true

		// Get MR object
		mr, err := cache.getOrFetchMR(client, projectPath, commit.PRNumber)
		if err != nil {
			slog.Warn("Failed to fetch MR for guidance extraction", "mr_iid", commit.PRNumber, "error", err)
			continue
		}

		if mr == nil {
			continue
		}

		// Extract guidance from this MR
		slog.Debug("Extracting user guidance from MR", "mr_iid", commit.PRNumber)
		guidance, err := extractUserGuidance(client, projectPath, mr, cache)
		if err != nil {
			slog.Warn("Failed to extract user guidance", "mr_iid", commit.PRNumber, "error", err)
		} else if len(guidance) > 0 {
			allGuidance = append(allGuidance, guidance...)
		}
	}

	slog.Debug("User guidance extraction complete", "items", len(allGuidance))
	return allGuidance, nil
}

// extractUserGuidance extracts all user guidance from a MR's notes
func extractUserGuidance(client *gitlab.Client, projectPath string, mr *gitlab.MergeRequest, cache *mrCache) ([]types.UserGuidance, error) {
	if mr == nil {
		return nil, nil
	}

	mrIID := mr.IID
	var allGuidance []types.UserGuidance

	// Get all notes for this MR (cached)
	notes, err := cache.getOrFetchMRNotes(client, projectPath, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notes for MR !%d: %w", mrIID, err)
	}

	// Get MR approvers for authorization check (not cached, relatively cheap)
	approvers, err := getMRApprovers(client, projectPath, mrIID)
	if err != nil {
		slog.Warn("Failed to get MR approvers", "mr_iid", mrIID, "error", err)
		approvers = []string{} // Continue without approvers
	}

	// Process each note
	for _, note := range notes {
		if note == nil || note.Body == "" || note.Author.Username == "" {
			continue
		}

		// Extract user guidance using shared parser
		guidanceContent, found := types.ParseUserGuidance(note.Body)
		if !found {
			continue
		}

		// Check if author is authorized (MR author or approver)
		mrAuthor := ""
		if mr.Author != nil {
			mrAuthor = mr.Author.Username
		}
		isAuthorized := isAuthorized(note.Author.Username, mrAuthor, approvers)

		guidance := types.UserGuidance{
			Content:      guidanceContent,
			Author:       note.Author.Username,
			Date:         *note.CreatedAt,
			CommentURL:   "", // GitLab notes don't have direct URLs in the API
			IsAuthorized: isAuthorized,
		}

		slog.Debug("Found user guidance in MR note", "mr_iid", mrIID, "author", note.Author.Username, "authorized", isAuthorized)
		allGuidance = append(allGuidance, guidance)
	}

	return allGuidance, nil
}

// getMRApprovers returns the list of usernames who approved the MR
func getMRApprovers(client *gitlab.Client, projectPath string, mrIID int) ([]string, error) {
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

// isAuthorized checks if a user is authorized to provide guidance (MR author or approver)
func isAuthorized(username, mrAuthor string, approvers []string) bool {
	// Check if user is MR author
	if username == mrAuthor {
		return true
	}

	// Check if user is an approver
	for _, approver := range approvers {
		if username == approver {
			return true
		}
	}

	return false
}

// Cache method for user guidance extraction

func (c *mrCache) getOrFetchMRNotes(client *gitlab.Client, projectPath string, mrIID int) ([]*gitlab.Note, error) {
	key := cacheKey(projectPath, mrIID)

	// Check cache first (read lock)
	c.mu.RLock()
	if notes, exists := c.mrNotes[key]; exists {
		c.mu.RUnlock()
		slog.Debug("Using cached MR notes", "mr_iid", mrIID, "count", len(notes))
		return notes, nil
	}
	c.mu.RUnlock()

	// Cache miss - fetch from API with pagination
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

	slog.Debug("GitLab API response", "mr_iid", mrIID, "notes", len(allNotes))

	// Cache the result (write lock)
	c.mu.Lock()
	c.mrNotes[key] = allNotes
	c.mu.Unlock()

	return allNotes, nil
}
