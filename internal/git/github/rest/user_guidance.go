package rest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"

	"github.com/google/go-github/v80/github"
	"golang.org/x/sync/errgroup"
)

// fetchUserGuidance extracts user guidance from all PRs using REST API
// The cache parameter allows reusing PR objects already fetched during diff augmentation
func fetchUserGuidance(ctx context.Context, client *github.Client, owner, repo string, comparison *types.Comparison, cache *prCache) ([]types.UserGuidance, error) {
	if comparison == nil || len(comparison.Commits) == 0 {
		return []types.UserGuidance{}, nil
	}

	slog.Debug("Extracting user guidance from comparison", "commits", len(comparison.Commits))

	var allGuidance []types.UserGuidance

	// Track which PRs we've already processed to avoid duplicates
	processedPRs := make(map[int64]bool)

	// Extract guidance from each unique PR
	for _, commit := range comparison.Commits {
		if commit.PRNumber == 0 || processedPRs[commit.PRNumber] {
			continue
		}

		processedPRs[commit.PRNumber] = true

		// Get PR object (uses cache populated during diff augmentation)
		pr, err := cache.getOrFetchPR(ctx, client, owner, repo, int(commit.PRNumber))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch PR #%d for guidance extraction: %w", commit.PRNumber, err)
		}

		if pr == nil {
			continue
		}

		// Extract guidance from this PR
		slog.Debug("Extracting user guidance from PR", "pr", commit.PRNumber)
		guidance, err := extractUserGuidance(ctx, client, owner, repo, pr)
		if err != nil {
			return nil, fmt.Errorf("failed to extract user guidance from PR #%d: %w", commit.PRNumber, err)
		}

		if len(guidance) > 0 {
			allGuidance = append(allGuidance, guidance...)
		}
	}

	slog.Debug("User guidance extraction complete", "items", len(allGuidance))
	return allGuidance, nil
}

// extractUserGuidance extracts all user guidance from a PR's comments
// Fetches issue comments and review comments in parallel for better performance
func extractUserGuidance(ctx context.Context, client *github.Client, owner, repo string, pr *github.PullRequest) ([]types.UserGuidance, error) {
	if pr == nil {
		return nil, nil
	}

	prNumber := pr.GetNumber()

	// Fetch issue comments and review comments in parallel
	// Use a separate variable for errgroup context to avoid shadowing the outer ctx
	// (errgroup's context is canceled after Wait() returns, but we still need ctx for processComment)
	g, gCtx := errgroup.WithContext(ctx)

	var issueComments []*github.IssueComment
	var reviewComments []*github.PullRequestComment

	g.Go(func() error {
		var err error
		issueComments, err = fetchAllPaginated(gCtx,
			func(ctx context.Context, opts *github.ListOptions) ([]*github.IssueComment, *github.Response, error) {
				return client.Issues.ListComments(ctx, owner, repo, prNumber, &github.IssueListCommentsOptions{ListOptions: *opts})
			},
		)
		if err != nil {
			return fmt.Errorf("failed to get comments for PR #%d: %w", prNumber, err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		reviewComments, err = fetchAllPaginated(gCtx,
			func(ctx context.Context, opts *github.ListOptions) ([]*github.PullRequestComment, *github.Response, error) {
				return client.PullRequests.ListComments(ctx, owner, repo, prNumber, &github.PullRequestListCommentsOptions{ListOptions: *opts})
			},
		)
		if err != nil {
			return fmt.Errorf("failed to get review comments for PR #%d: %w", prNumber, err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Process comments sequentially - use original ctx (not gCtx which is now canceled)
	var allGuidance []types.UserGuidance

	for _, comment := range issueComments {
		if !isValidIssueComment(comment) {
			continue
		}

		guidance, err := processComment(
			ctx,
			comment.GetBody(),
			comment.GetUser().GetLogin(),
			comment.GetCreatedAt().Time,
			comment.GetHTMLURL(),
			client, pr, prNumber, "issue comment",
		)
		if err != nil {
			return nil, err
		}
		if guidance != nil {
			allGuidance = append(allGuidance, *guidance)
		}
	}

	for _, comment := range reviewComments {
		if !isValidReviewComment(comment) {
			continue
		}

		guidance, err := processComment(
			ctx,
			comment.GetBody(),
			comment.GetUser().GetLogin(),
			comment.GetCreatedAt().Time,
			comment.GetHTMLURL(),
			client, pr, prNumber, "review comment",
		)
		if err != nil {
			return nil, err
		}
		if guidance != nil {
			allGuidance = append(allGuidance, *guidance)
		}
	}

	return allGuidance, nil
}

// processComment processes a single comment and returns user guidance if found
func processComment(ctx context.Context, body, author string, date time.Time, url string, client *github.Client, pr *github.PullRequest, prNumber int, commentType string) (*types.UserGuidance, error) {
	guidanceContent, found := shared.ParseUserGuidance(body)
	if !found {
		return nil, nil
	}

	isAuthorized, err := isAuthorized(ctx, client, pr, author)
	if err != nil {
		return nil, fmt.Errorf("failed to check authorization for guidance author %s: %w", author, err)
	}

	slog.Debug("Found user guidance in "+commentType, "pr", prNumber, "author", author, "authorized", isAuthorized)

	return &types.UserGuidance{
		Content:      guidanceContent,
		Author:       author,
		Date:         date,
		CommentURL:   url,
		IsAuthorized: isAuthorized,
	}, nil
}

// isValidIssueComment checks if an issue comment has all required fields
func isValidIssueComment(comment *github.IssueComment) bool {
	if comment == nil || comment.GetBody() == "" {
		return false
	}
	if comment.GetUser() == nil || comment.GetUser().GetLogin() == "" {
		return false
	}
	return comment.GetHTMLURL() != ""
}

// isValidReviewComment checks if a review comment has all required fields
func isValidReviewComment(comment *github.PullRequestComment) bool {
	if comment == nil || comment.GetBody() == "" {
		return false
	}
	if comment.GetUser() == nil || comment.GetUser().GetLogin() == "" {
		return false
	}
	return comment.GetHTMLURL() != ""
}

// isAuthorized checks if a user is authorized to provide guidance
// Authorization criteria:
// 1. User is the PR author, OR
// 2. User approved the PR with proper repository permissions (OWNER, MEMBER, or COLLABORATOR)
//
// Note: GitHub requires checking AuthorAssociation because anyone can submit reviews,
// so we verify the reviewer has meaningful authority in the repository.
func isAuthorized(ctx context.Context, client *github.Client, pr *github.PullRequest, username string) (bool, error) {
	// Check if user is the PR author
	if pr.User != nil && pr.User.GetLogin() == username {
		slog.Debug("User authorized as PR author", "user", username, "pr", pr.GetNumber())
		return true, nil
	}

	// Fetch PR reviews
	owner := pr.GetBase().GetRepo().GetOwner().GetLogin()
	repo := pr.GetBase().GetRepo().GetName()
	prNumber := pr.GetNumber()

	reviews, err := fetchAllPaginated(ctx,
		func(ctx context.Context, opts *github.ListOptions) ([]*github.PullRequestReview, *github.Response, error) {
			return client.PullRequests.ListReviews(ctx, owner, repo, prNumber, opts)
		},
	)
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
	// Only OWNER, MEMBER, or COLLABORATOR associations are considered authorized
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

// fetchAllPaginated is a generic helper that fetches all pages of GitHub API results
func fetchAllPaginated[T any](ctx context.Context, fetcher func(context.Context, *github.ListOptions) ([]T, *github.Response, error)) ([]T, error) {
	var allItems []T
	opts := &github.ListOptions{
		PerPage: 100,
		Page:    1,
	}

	for {
		items, resp, err := fetcher(ctx, opts)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allItems, nil
}
