package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v76/github"
)

// githubCache caches GitHub API responses to avoid duplicate API calls
// Uses composite keys "owner/repo/identifier" to handle multiple repositories
type githubCache struct {
	commitToPR       map[string]int                          // "owner/repo/SHA" → PR number
	prs              map[string]*github.PullRequest          // "owner/repo/123" → PR object
	prIssueComments  map[string][]*github.IssueComment       // "owner/repo/123" → discussion comments
	prReviewComments map[string][]*github.PullRequestComment // "owner/repo/123" → review comments
	prReviews        map[string][]*github.PullRequestReview  // "owner/repo/123" → reviews
}

// newGithubCache creates a new GitHub API cache with all maps initialized
func newGithubCache() *githubCache {
	return &githubCache{
		commitToPR:       make(map[string]int),
		prs:              make(map[string]*github.PullRequest),
		prIssueComments:  make(map[string][]*github.IssueComment),
		prReviewComments: make(map[string][]*github.PullRequestComment),
		prReviews:        make(map[string][]*github.PullRequestReview),
	}
}

// prCacheKey creates a composite cache key for PR-related data
func prCacheKey(owner, repo string, prNumber int) string {
	return fmt.Sprintf("%s/%s/%d", owner, repo, prNumber)
}

// commitCacheKey creates a composite cache key for commit-related data
func commitCacheKey(owner, repo, commitSHA string) string {
	return fmt.Sprintf("%s/%s/%s", owner, repo, commitSHA)
}

// getOrFetchPRForCommit returns cached PR number for commit or fetches it from GitHub API
func (c *githubCache) getOrFetchPRForCommit(client *github.Client, owner, repo, commitSHA string) (int, error) {
	cacheKey := commitCacheKey(owner, repo, commitSHA)

	// Return cached PR number if available
	if prNumber, exists := c.commitToPR[cacheKey]; exists {
		slog.Debug("Using cached commit→PR mapping", "commit", commitSHA, "pr", prNumber)
		return prNumber, nil
	}

	// Fetch PR number from GitHub API
	prs, resp, err := client.PullRequests.ListPullRequestsWithCommit(context.Background(), owner, repo, commitSHA, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to find PRs for commit %s: %w", commitSHA, err)
	}

	slog.Debug("GitHub API response", "commit", commitSHA, "found_prs", len(prs), "rate_limit_remaining", resp.Rate.Remaining)

	// Find merged PRs and warn if multiple exist
	var mergedPRs []int
	for _, pr := range prs {
		if !pr.GetMergedAt().IsZero() {
			mergedPRs = append(mergedPRs, pr.GetNumber())
		}
	}

	// Handle multiple merged PRs
	if len(mergedPRs) > 1 {
		slog.Warn("Multiple merged PRs found for commit (using first)",
			"commit", commitSHA,
			"pr_count", len(mergedPRs),
			"prs", mergedPRs,
			"selected_pr", mergedPRs[0])
	}

	// Return first merged PR if found
	if len(mergedPRs) > 0 {
		slog.Debug("Found merged PR", "commit", commitSHA, "pr", mergedPRs[0])
		c.commitToPR[cacheKey] = mergedPRs[0]
		return mergedPRs[0], nil
	}

	// No merged PR found for this commit
	slog.Debug("No merged PR found", "commit", commitSHA, "total_prs_checked", len(prs))
	c.commitToPR[cacheKey] = 0 // Cache the fact that no PR was found
	return 0, nil              // No PR found
}

// getOrFetchPR returns cached PR object or fetches it from GitHub API
func (c *githubCache) getOrFetchPR(client *github.Client, owner, repo string, prNumber int) (*github.PullRequest, error) {
	// Handle commits without PRs (direct commits to master/main)
	if prNumber == 0 {
		return nil, nil
	}

	cacheKey := prCacheKey(owner, repo, prNumber)

	// Return cached PR if available
	if pr, exists := c.prs[cacheKey]; exists {
		slog.Debug("Using cached PR object", "pr", prNumber)
		return pr, nil
	}

	// Fetch PR from GitHub API
	pr, resp, err := client.PullRequests.Get(context.Background(), owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "rate_limit_remaining", resp.Rate.Remaining)

	// Cache the PR object
	c.prs[cacheKey] = pr
	slog.Debug("Cached PR object", "pr", prNumber)

	return pr, nil
}

// getOrFetchIssueComments returns cached PR discussion comments or fetches them from GitHub API
func (c *githubCache) getOrFetchIssueComments(client *github.Client, owner, repo string, prNumber int) ([]*github.IssueComment, error) {
	cacheKey := prCacheKey(owner, repo, prNumber)

	// Return cached comments if available
	if comments, exists := c.prIssueComments[cacheKey]; exists {
		slog.Debug("Using cached issue comments", "pr", prNumber, "count", len(comments))
		return comments, nil
	}

	// Fetch comments from GitHub API
	comments, resp, err := client.Issues.ListComments(context.Background(), owner, repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "issue_comments", len(comments), "rate_limit_remaining", resp.Rate.Remaining)

	// Cache the comments
	c.prIssueComments[cacheKey] = comments
	slog.Debug("Cached issue comments", "pr", prNumber, "count", len(comments))

	return comments, nil
}

// getOrFetchReviewComments returns cached PR review comments or fetches them from GitHub API
func (c *githubCache) getOrFetchReviewComments(client *github.Client, owner, repo string, prNumber int) ([]*github.PullRequestComment, error) {
	cacheKey := prCacheKey(owner, repo, prNumber)

	// Return cached comments if available
	if comments, exists := c.prReviewComments[cacheKey]; exists {
		slog.Debug("Using cached review comments", "pr", prNumber, "count", len(comments))
		return comments, nil
	}

	// Fetch comments from GitHub API
	comments, resp, err := client.PullRequests.ListComments(context.Background(), owner, repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get review comments for PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "review_comments", len(comments), "rate_limit_remaining", resp.Rate.Remaining)

	// Cache the comments
	c.prReviewComments[cacheKey] = comments
	slog.Debug("Cached review comments", "pr", prNumber, "count", len(comments))

	return comments, nil
}

// getOrFetchReviews returns cached PR reviews or fetches them from GitHub API
func (c *githubCache) getOrFetchReviews(client *github.Client, owner, repo string, prNumber int) ([]*github.PullRequestReview, error) {
	cacheKey := prCacheKey(owner, repo, prNumber)

	// Return cached reviews if available
	if reviews, exists := c.prReviews[cacheKey]; exists {
		slog.Debug("Using cached PR reviews", "pr", prNumber, "count", len(reviews))
		return reviews, nil
	}

	// Fetch reviews from GitHub API
	reviews, resp, err := client.PullRequests.ListReviews(context.Background(), owner, repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews for PR #%d: %w", prNumber, err)
	}

	slog.Debug("GitHub API response", "pr", prNumber, "reviews", len(reviews), "rate_limit_remaining", resp.Rate.Remaining)

	// Cache the reviews
	c.prReviews[cacheKey] = reviews
	slog.Debug("Cached PR reviews", "pr", prNumber, "count", len(reviews))

	return reviews, nil
}
