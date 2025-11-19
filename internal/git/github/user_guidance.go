package github

import (
	"fmt"
	"log/slog"

	"release-confidence-score/internal/shared"

	"github.com/google/go-github/v76/github"
)

// gitHubComment is an interface for GitHub comment types
type gitHubComment interface {
	GetBody() string
	GetUser() *github.User
	GetCreatedAt() github.Timestamp
	GetHTMLURL() string
}

// GetPRUserGuidance extracts all user guidance from a PR's comments
// Returns empty slice if no guidance found
// Accepts a PR object to avoid duplicate API calls - the PR should be cached
// Uses cache for comments and reviews to avoid duplicate API calls
func GetPRUserGuidance(client *github.Client, owner, repo string, pr *github.PullRequest, githubCache *githubCache) ([]shared.UserGuidance, error) {
	if pr == nil {
		return nil, nil
	}

	prNumber := pr.GetNumber()

	var allUserGuidance []shared.UserGuidance

	// Check issue comments (PR discussion) - most likely location for user guidance
	comments, err := githubCache.getOrFetchIssueComments(client, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for PR #%d: %w", prNumber, err)
	}

	// Extract ALL user guidance from issue comments (both authorized and unauthorized)
	issueGuidance := parseUserGuidanceFromIssueComments(client, pr, comments, githubCache)
	allUserGuidance = append(allUserGuidance, issueGuidance...)

	// Also check PR review comments (less common for RCS, but possible)
	reviewComments, err := githubCache.getOrFetchReviewComments(client, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get review comments for PR #%d: %w", prNumber, err)
	}

	// Extract ALL user guidance from review comments (both authorized and unauthorized)
	reviewGuidance := parseUserGuidanceFromReviewComments(client, pr, reviewComments, githubCache)
	allUserGuidance = append(allUserGuidance, reviewGuidance...)

	// Log summary
	authorizedCount := 0
	for _, guidance := range allUserGuidance {
		if guidance.IsAuthorized {
			authorizedCount++
		}
	}

	slog.Debug("User guidance summary",
		"pr", prNumber,
		"total_guidance", len(allUserGuidance),
		"authorized", authorizedCount)

	return allUserGuidance, nil
}

// parseUserGuidanceFromIssueComments extracts ALL user guidance from GitHub issue comments with full metadata
func parseUserGuidanceFromIssueComments(client *github.Client, pr *github.PullRequest, comments []*github.IssueComment, githubCache *githubCache) []shared.UserGuidance {
	return parseGuidanceFromComments(client, pr, comments, githubCache)
}

// parseUserGuidanceFromReviewComments extracts ALL user guidance from GitHub review comments with full metadata
func parseUserGuidanceFromReviewComments(client *github.Client, pr *github.PullRequest, comments []*github.PullRequestComment, githubCache *githubCache) []shared.UserGuidance {
	return parseGuidanceFromComments(client, pr, comments, githubCache)
}

// parseGuidanceFromComments is a generic helper that extracts guidance from any GitHub comment type
func parseGuidanceFromComments[T gitHubComment](client *github.Client, pr *github.PullRequest, comments []T, githubCache *githubCache) []shared.UserGuidance {
	var allGuidance []shared.UserGuidance

	for _, comment := range comments {
		guidanceContent, found := shared.ParseUserGuidance(comment.GetBody())
		if !found {
			continue
		}

		author := comment.GetUser().GetLogin()

		// Check authorization
		isAuthorized, err := isPRAuthorOrApprover(client, pr, author, githubCache)
		if err != nil {
			slog.Warn("Failed to check permissions for user guidance",
				"author", author,
				"guidance", guidanceContent,
				"error", err)
			isAuthorized = false
		}

		// Create UserGuidance with metadata
		guidance := shared.UserGuidance{
			Content:      guidanceContent,
			Author:       author,
			Date:         comment.GetCreatedAt().Time,
			CommentURL:   comment.GetHTMLURL(),
			IsAuthorized: isAuthorized,
		}

		slog.Debug("Found GitHub user guidance",
			"author", guidance.Author,
			"guidance", guidance.Content,
			"authorized", guidance.IsAuthorized)

		allGuidance = append(allGuidance, guidance)
	}

	return allGuidance
}

// isPRAuthorOrApprover checks if the user is either the PR author or one of the meaningful approvers
func isPRAuthorOrApprover(client *github.Client, pr *github.PullRequest, username string, githubCache *githubCache) (bool, error) {

	// Check if user is the PR author
	if pr.User != nil && pr.User.GetLogin() == username {
		slog.Debug("User authorized as PR author", "user", username, "pr", pr.GetNumber())
		return true, nil
	}

	// Get PR reviews to check for approvals (cached to avoid duplicate API calls)
	owner := pr.GetBase().GetRepo().GetOwner().GetLogin()
	repo := pr.GetBase().GetRepo().GetName()
	prNumber := pr.GetNumber()

	reviews, err := githubCache.getOrFetchReviews(client, owner, repo, prNumber)
	if err != nil {
		return false, fmt.Errorf("failed to get reviews for PR #%d: %w", prNumber, err)
	}

	// Find the user's latest review (users can change their review)
	var latestReview *github.PullRequestReview
	for _, review := range reviews {
		if review.User == nil || review.SubmittedAt == nil || review.User.GetLogin() != username {
			continue
		}

		if latestReview == nil || review.SubmittedAt.After(latestReview.SubmittedAt.Time) {
			latestReview = review
		}
	}

	// Check if user approved with meaningful authority (OWNER, MEMBER, or COLLABORATOR)
	if latestReview != nil && latestReview.GetState() == "APPROVED" {
		association := latestReview.GetAuthorAssociation()
		if association == "OWNER" || association == "MEMBER" || association == "COLLABORATOR" {
			slog.Debug("User authorized as meaningful PR approver",
				"user", username,
				"pr", pr.GetNumber(),
				"association", association)
			return true, nil
		}
	}

	slog.Debug("User not authorized (not author or approver)", "user", username, "pr", pr.GetNumber())
	return false, nil
}
