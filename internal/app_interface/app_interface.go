package app_interface

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

const (
	// projectID is the GitLab project ID for app-interface
	projectID = "service/app-interface"
	// botUsername is the username of the bot that posts diff URLs
	botUsername = "devtools-bot"
	// diffsMarker is the text marker that identifies the bot's diff comment
	diffsMarker = "Diffs:"
)

// Pre-compiled regex for URL extraction from bot comments
// Matches URLs on lines starting with "- " (dash-space prefix)
// Uses multiline mode ((?m)) so ^ matches start of each line
var urlRegex = regexp.MustCompile(`(?m)^- (https?://\S+)$`)

// GetDiffURLsAndUserGuidance fetches merge request notes and extracts diff URLs and user guidance
func GetDiffURLsAndUserGuidance(client *gitlabapi.Client, cfg *config.Config, mergeRequestIID int) ([]string, []types.UserGuidance, error) {
	notes, err := getAllMergeRequestNotes(client, mergeRequestIID)
	if err != nil {
		return nil, nil, err
	}

	diffURLs, err := extractDiffURLsFromBot(notes)
	if err != nil {
		return nil, nil, err
	}

	userGuidance := extractUserGuidance(cfg, mergeRequestIID, notes)

	return diffURLs, userGuidance, nil
}

// getAllMergeRequestNotes fetches all notes for a merge request with automatic pagination
func getAllMergeRequestNotes(client *gitlabapi.Client, mergeRequestIID int) ([]*gitlabapi.Note, error) {
	orderBy := "created_at"
	sort := "desc"
	opts := &gitlabapi.ListMergeRequestNotesOptions{
		// Use maximum PerPage value of 100 to minimize API calls
		// See: https://docs.gitlab.com/api/rest/#pagination
		ListOptions: gitlabapi.ListOptions{PerPage: 100},
		OrderBy:     &orderBy,
		Sort:        &sort,
	}

	var notes []*gitlabapi.Note
	for {
		pageNotes, resp, err := client.Notes.ListMergeRequestNotes(projectID, mergeRequestIID, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get merge request notes: %w", err)
		}

		notes = append(notes, pageNotes...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return notes, nil
}

// extractDiffURLsFromBot finds the devtools-bot comment and extracts diff URLs
func extractDiffURLsFromBot(notes []*gitlabapi.Note) ([]string, error) {
	for _, note := range notes {
		if note.Author.Username != botUsername || !strings.HasPrefix(note.Body, diffsMarker) {
			continue
		}

		// Extract URLs using submatch to get captured group (the URL without "- " prefix)
		matches := urlRegex.FindAllStringSubmatch(note.Body, -1)
		var urls []string
		for _, match := range matches {
			if len(match) > 1 {
				urls = append(urls, match[1]) // match[1] is the captured URL
			}
		}

		if len(urls) > 0 {
			slog.Debug("Found devtools-bot diff URLs", "count", len(urls))
			return urls, nil
		}
	}

	return nil, fmt.Errorf("no devtools-bot URLs found")
}

// extractUserGuidance extracts user guidance from merge request notes with full metadata
func extractUserGuidance(cfg *config.Config, mergeRequestIID int, notes []*gitlabapi.Note) []types.UserGuidance {
	var allGuidance []types.UserGuidance

	// Build the merge request URL from config
	mrURL := fmt.Sprintf("%s/%s/-/merge_requests/%d", cfg.GitLabBaseURL, projectID, mergeRequestIID)

	for _, note := range notes {
		guidanceContent, found := shared.ParseUserGuidance(note.Body)
		if !found {
			continue
		}

		// Guard against nil CreatedAt
		if note.CreatedAt == nil {
			slog.Warn("Skipping guidance with nil CreatedAt", "note_id", note.ID)
			continue
		}

		guidance := types.UserGuidance{
			Content:      guidanceContent,
			Author:       note.Author.Username,
			Date:         *note.CreatedAt,
			CommentURL:   fmt.Sprintf("%s#note_%d", mrURL, note.ID),
			IsAuthorized: true, // GitLab guidance from app-interface is always authorized
		}

		slog.Debug("Found app-interface user guidance", "author", guidance.Author)
		allGuidance = append(allGuidance, guidance)
	}

	return allGuidance
}

// PostReportToMR posts the release confidence score report to a GitLab merge request
func PostReportToMR(client *gitlabapi.Client, report string, mrIID int) error {
	opts := &gitlabapi.CreateMergeRequestNoteOptions{
		Body: &report,
	}

	_, _, err := client.Notes.CreateMergeRequestNote(projectID, mrIID, opts)
	if err != nil {
		return fmt.Errorf("failed to post comment to MR: %w", err)
	}

	return nil
}
