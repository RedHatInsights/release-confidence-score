package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v79/github"
	"release-confidence-score/internal/git/types"
)

// FetchUserGuidance extracts user guidance from all PRs in the comparison
func FetchUserGuidance(client *github.Client, owner, repo string, comparison *types.Comparison) ([]types.UserGuidance, error) {
	if comparison == nil || len(comparison.Commits) == 0 {
		return []types.UserGuidance{}, nil
	}

	slog.Debug("Extracting user guidance from comparison", "commits", len(comparison.Commits))

	// Create cache to avoid duplicate API calls
	cache := newPRCache()
	var allGuidance []types.UserGuidance

	// Track which PRs we've already processed to avoid duplicates
	processedPRs := make(map[int]bool)

	// Extract guidance from each unique PR
	for _, commit := range comparison.Commits {
		if commit.PRNumber == 0 || processedPRs[commit.PRNumber] {
			continue
		}

		processedPRs[commit.PRNumber] = true

		// Get PR object
		pr, err := cache.getOrFetchPR(client, owner, repo, commit.PRNumber)
		if err != nil {
			slog.Warn("Failed to fetch PR for guidance extraction", "pr", commit.PRNumber, "error", err)
			continue
		}

		if pr == nil {
			continue
		}

		// Extract guidance from this PR
		slog.Debug("Extracting user guidance from PR", "pr", commit.PRNumber)
		guidance, err := extractUserGuidance(client, owner, repo, pr, cache)
		if err != nil {
			slog.Warn("Failed to extract user guidance", "pr", commit.PRNumber, "error", err)
		} else if len(guidance) > 0 {
			allGuidance = append(allGuidance, guidance...)
		}
	}

	slog.Debug("User guidance extraction complete", "items", len(allGuidance))
	return allGuidance, nil
}

// extractUserGuidance extracts all user guidance from a PR's comments
func extractUserGuidance(client *github.Client, owner, repo string, pr *github.PullRequest, cache *prCache) ([]types.UserGuidance, error) {
	if pr == nil {
		return nil, nil
	}

	prNumber := pr.GetNumber()
	var allGuidance []types.UserGuidance

	// Check issue comments (PR discussion)
	issueComments, err := cache.getOrFetchIssueComments(client, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for PR #%d: %w", prNumber, err)
	}

	for _, comment := range issueComments {
		guidanceContent, found := types.ParseUserGuidance(comment.GetBody())
		if !found {
			continue
		}

		author := comment.GetUser().GetLogin()
		isAuthorized, err := isAuthorized(client, pr, author, cache)
		if err != nil {
			slog.Warn("Failed to check authorization for guidance", "author", author, "error", err)
			isAuthorized = false
		}

		guidance := types.UserGuidance{
			Content:      guidanceContent,
			Author:       author,
			Date:         comment.GetCreatedAt().Time,
			CommentURL:   comment.GetHTMLURL(),
			IsAuthorized: isAuthorized,
		}

		slog.Debug("Found user guidance in issue comment", "pr", prNumber, "author", author, "authorized", isAuthorized)
		allGuidance = append(allGuidance, guidance)
	}

	// Check review comments
	reviewComments, err := cache.getOrFetchReviewComments(client, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get review comments for PR #%d: %w", prNumber, err)
	}

	for _, comment := range reviewComments {
		guidanceContent, found := types.ParseUserGuidance(comment.GetBody())
		if !found {
			continue
		}

		author := comment.GetUser().GetLogin()
		isAuthorized, err := isAuthorized(client, pr, author, cache)
		if err != nil {
			slog.Warn("Failed to check authorization for guidance", "author", author, "error", err)
			isAuthorized = false
		}

		guidance := types.UserGuidance{
			Content:      guidanceContent,
			Author:       author,
			Date:         comment.GetCreatedAt().Time,
			CommentURL:   comment.GetHTMLURL(),
			IsAuthorized: isAuthorized,
		}

		slog.Debug("Found user guidance in review comment", "pr", prNumber, "author", author, "authorized", isAuthorized)
		allGuidance = append(allGuidance, guidance)
	}

	return allGuidance, nil
}

// isAuthorized checks if a user is authorized to provide guidance (PR author or approver)
func isAuthorized(client *github.Client, pr *github.PullRequest, username string, cache *prCache) (bool, error) {
	// Check if user is the PR author
	if pr.User != nil && pr.User.GetLogin() == username {
		slog.Debug("User authorized as PR author", "user", username, "pr", pr.GetNumber())
		return true, nil
	}

	// Get PR reviews (cached)
	owner := pr.GetBase().GetRepo().GetOwner().GetLogin()
	repo := pr.GetBase().GetRepo().GetName()
	prNumber := pr.GetNumber()

	reviews, err := cache.getOrFetchReviews(client, owner, repo, prNumber)
	if err != nil {
		return false, fmt.Errorf("failed to get reviews for PR #%d: %w", prNumber, err)
	}

	// Find the user's latest review
	var latestReview *github.PullRequestReview
	for _, review := range reviews {
		if review.User == nil || review.SubmittedAt == nil || review.User.GetLogin() != username {
			continue
		}

		if latestReview == nil || review.SubmittedAt.After(latestReview.SubmittedAt.Time) {
			latestReview = review
		}
	}

	// Check if user approved with meaningful authority
	if latestReview != nil && latestReview.GetState() == "APPROVED" {
		association := latestReview.GetAuthorAssociation()
		if association == "OWNER" || association == "MEMBER" || association == "COLLABORATOR" {
			slog.Debug("User authorized as approver", "user", username, "pr", prNumber, "association", association)
			return true, nil
		}
	}

	slog.Debug("User not authorized", "user", username, "pr", prNumber)
	return false, nil
}

// Cache methods for user guidance extraction

func (c *prCache) getOrFetchIssueComments(client *github.Client, owner, repo string, prNumber int) ([]*github.IssueComment, error) {
	key := cacheKey(owner, repo, prNumber)

	if comments, exists := c.prIssueComments[key]; exists {
		slog.Debug("Using cached issue comments", "pr", prNumber, "count", len(comments))
		return comments, nil
	}

	comments, resp, err := client.Issues.ListComments(context.Background(), owner, repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "comments", len(comments), "rate_limit_remaining", resp.Rate.Remaining)
	c.prIssueComments[key] = comments
	return comments, nil
}

func (c *prCache) getOrFetchReviewComments(client *github.Client, owner, repo string, prNumber int) ([]*github.PullRequestComment, error) {
	key := cacheKey(owner, repo, prNumber)

	if comments, exists := c.prReviewComments[key]; exists {
		slog.Debug("Using cached review comments", "pr", prNumber, "count", len(comments))
		return comments, nil
	}

	comments, resp, err := client.PullRequests.ListComments(context.Background(), owner, repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get review comments for PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "review_comments", len(comments), "rate_limit_remaining", resp.Rate.Remaining)
	c.prReviewComments[key] = comments
	return comments, nil
}

func (c *prCache) getOrFetchReviews(client *github.Client, owner, repo string, prNumber int) ([]*github.PullRequestReview, error) {
	key := cacheKey(owner, repo, prNumber)

	if reviews, exists := c.prReviews[key]; exists {
		slog.Debug("Using cached PR reviews", "pr", prNumber, "count", len(reviews))
		return reviews, nil
	}

	reviews, resp, err := client.PullRequests.ListReviews(context.Background(), owner, repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews for PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "reviews", len(reviews), "rate_limit_remaining", resp.Rate.Remaining)
	c.prReviews[key] = reviews
	return reviews, nil
}
