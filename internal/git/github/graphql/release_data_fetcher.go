package graphql

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	githubapi "github.com/google/go-github/v80/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/sync/errgroup"
	"release-confidence-score/internal/config"
	ghshared "release-confidence-score/internal/git/github/shared"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// Fetcher implements GitProvider using GitHub's GraphQL API where beneficial.
// Uses REST for compare (no GraphQL equivalent) and GraphQL for PR data.
type Fetcher struct {
	restClient    *githubapi.Client
	graphqlClient *githubv4.Client
	config        *config.Config
}

// NewFetcher creates a new GitHub GraphQL-based fetcher
func NewFetcher(cfg *config.Config) *Fetcher {
	return &Fetcher{
		restClient:    ghshared.NewRESTClient(cfg.GitHubToken),
		graphqlClient: newClient(cfg.GitHubToken),
		config:        cfg,
	}
}

// Name returns the platform name
func (f *Fetcher) Name() string {
	return "GitHub"
}

// IsCompareURL checks if a URL is a valid GitHub compare URL
func (f *Fetcher) IsCompareURL(url string) bool {
	return ghshared.CompareURLRegex.MatchString(url)
}

// FetchReleaseData fetches all release data for a GitHub compare URL
func (f *Fetcher) FetchReleaseData(ctx context.Context, compareURL string) (*types.Comparison, []types.UserGuidance, *types.Documentation, error) {
	slog.Debug("Fetching GitHub release data via GraphQL", "url", compareURL)

	owner, repo, baseCommit, headCommit, err := ghshared.ParseCompareURL(compareURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse GitHub compare URL: %w", err)
	}

	slog.Debug("Parsed compare URL", "owner", owner, "repo", repo, "base", baseCommit, "head", headCommit)

	g, gCtx := errgroup.WithContext(ctx)

	var comparison *types.Comparison
	var userGuidance []types.UserGuidance
	var documentation *types.Documentation

	// Fetch diff and user guidance
	g.Go(func() error {
		var err error
		comparison, err = f.fetchDiff(gCtx, owner, repo, baseCommit, headCommit, compareURL)
		if err != nil {
			return fmt.Errorf("failed to fetch comparison: %w", err)
		}

		userGuidance, err = f.fetchUserGuidance(gCtx, owner, repo, comparison)
		if err != nil {
			return fmt.Errorf("failed to fetch user guidance: %w", err)
		}
		return nil
	})

	// Fetch documentation (independent)
	g.Go(func() error {
		docSource := ghshared.NewDocumentationSource(f.restClient, owner, repo)
		baseRepo := types.Repository{
			Owner: owner,
			Name:  repo,
			URL:   ghshared.ExtractRepoURL(compareURL),
		}
		docFetcher := shared.NewDocumentationFetcher(docSource, baseRepo, f.config)

		var err error
		documentation, err = docFetcher.FetchAllDocs(gCtx)
		if err != nil {
			return fmt.Errorf("failed to fetch documentation: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, nil, nil, err
	}

	slog.Debug("Release data fetched successfully via GraphQL",
		"commit_entries", len(comparison.Commits),
		"user_guidance_items", len(userGuidance),
		"files", len(comparison.Files),
		"has_documentation", documentation != nil)

	return comparison, userGuidance, documentation, nil
}

// fetchDiff uses REST for compare and GraphQL for PR augmentation
func (f *Fetcher) fetchDiff(ctx context.Context, owner, repo, base, head, diffURL string) (*types.Comparison, error) {
	slog.Debug("Fetching comparison via REST, augmenting via GraphQL", "owner", owner, "repo", repo)

	ghComparison, allCommits, err := ghshared.FetchComparisonWithPagination(ctx, f.restClient, owner, repo, base, head)
	if err != nil {
		return nil, err
	}

	comparison := &types.Comparison{
		RepoURL: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		DiffURL: diffURL,
		Commits: make([]types.Commit, len(allCommits)),
		Files:   ghshared.ConvertFiles(ghComparison.Files),
		Stats:   ghshared.CalculateStats(ghComparison.Files),
	}

	for i, commit := range allCommits {
		comparison.Commits[i] = ghshared.BuildBasicCommitEntry(commit)
	}

	if err := f.augmentCommitsWithPRData(ctx, owner, repo, comparison.Commits); err != nil {
		slog.Warn("Failed to augment commits with PR data via GraphQL", "error", err)
	}

	return comparison, nil
}

func (f *Fetcher) augmentCommitsWithPRData(ctx context.Context, owner, repo string, commits []types.Commit) error {
	if len(commits) == 0 {
		return nil
	}

	slog.Debug("Augmenting commits with PR data via GraphQL", "count", len(commits))

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i := range commits {
		g.Go(func() error {
			prNumber, labels, err := f.getPRDataForCommit(gCtx, owner, repo, commits[i].SHA)
			if err != nil {
				slog.Debug("Failed to get PR for commit", "sha", commits[i].ShortSHA, "error", err)
				return nil
			}

			if prNumber == 0 {
				return nil
			}

			commits[i].PRNumber = int64(prNumber)
			commits[i].QETestingLabel = shared.ExtractQELabel(labels)
			slog.Debug("Augmented commit via GraphQL", "sha", commits[i].ShortSHA, "pr", prNumber, "qe_label", commits[i].QETestingLabel)
			return nil
		})
	}

	g.Wait()
	return nil
}

type commitPRWithLabelsQuery struct {
	Repository struct {
		Object struct {
			Commit struct {
				AssociatedPullRequests struct {
					Nodes []struct {
						Number   int
						MergedAt githubv4.DateTime
						Labels   struct {
							Nodes []struct {
								Name string
							}
						} `graphql:"labels(first: 100)"`
					}
				} `graphql:"associatedPullRequests(first: 10)"`
			} `graphql:"... on Commit"`
		} `graphql:"object(oid: $oid)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

func (f *Fetcher) getPRDataForCommit(ctx context.Context, owner, repo, sha string) (int, []string, error) {
	var query commitPRWithLabelsQuery
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
		"oid":   githubv4.GitObjectID(sha),
	}

	if err := f.graphqlClient.Query(ctx, &query, variables); err != nil {
		return 0, nil, err
	}

	for _, pr := range query.Repository.Object.Commit.AssociatedPullRequests.Nodes {
		if pr.MergedAt.IsZero() {
			continue
		}

		labels := make([]string, len(pr.Labels.Nodes))
		for i, label := range pr.Labels.Nodes {
			labels[i] = label.Name
		}

		return pr.Number, labels, nil
	}

	return 0, nil, nil
}

func (f *Fetcher) fetchUserGuidance(ctx context.Context, owner, repo string, comparison *types.Comparison) ([]types.UserGuidance, error) {
	if comparison == nil || len(comparison.Commits) == 0 {
		return []types.UserGuidance{}, nil
	}

	var prNumbers []int64
	seen := make(map[int64]bool)
	for _, commit := range comparison.Commits {
		if commit.PRNumber != 0 && !seen[commit.PRNumber] {
			seen[commit.PRNumber] = true
			prNumbers = append(prNumbers, commit.PRNumber)
		}
	}

	if len(prNumbers) == 0 {
		return []types.UserGuidance{}, nil
	}

	var mu sync.Mutex
	var allGuidance []types.UserGuidance

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for _, prNumber := range prNumbers {
		g.Go(func() error {
			guidance, err := f.fetchPRGuidance(gCtx, owner, repo, int(prNumber))
			if err != nil {
				return fmt.Errorf("failed to fetch guidance from PR #%d: %w", prNumber, err)
			}

			if len(guidance) > 0 {
				mu.Lock()
				allGuidance = append(allGuidance, guidance...)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	slog.Debug("User guidance extraction complete via GraphQL", "items", len(allGuidance))
	return allGuidance, nil
}

type prAuthorQuery struct {
	Repository struct {
		PullRequest struct {
			Author struct {
				Login string
			}
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type prCommentsQuery struct {
	Repository struct {
		PullRequest struct {
			Comments struct {
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
				Nodes []struct {
					Body      string
					Author    struct{ Login string }
					CreatedAt githubv4.DateTime
					URL       string
				}
			} `graphql:"comments(first: 100, after: $commentsCursor)"`
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type prReviewsQuery struct {
	Repository struct {
		PullRequest struct {
			Reviews struct {
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
				Nodes []struct {
					ID                githubv4.ID
					Author            struct{ Login string }
					State             string
					AuthorAssociation string
					SubmittedAt       githubv4.DateTime
					Comments          struct {
						PageInfo struct {
							HasNextPage bool
							EndCursor   githubv4.String
						}
						Nodes []struct {
							Body      string
							Author    struct{ Login string }
							CreatedAt githubv4.DateTime
							URL       string
						}
					} `graphql:"comments(first: 100)"`
				}
			} `graphql:"reviews(first: 100, after: $reviewsCursor)"`
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type reviewCommentsQuery struct {
	Node struct {
		PullRequestReview struct {
			Comments struct {
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
				Nodes []struct {
					Body      string
					Author    struct{ Login string }
					CreatedAt githubv4.DateTime
					URL       string
				}
			} `graphql:"comments(first: 100, after: $cursor)"`
		} `graphql:"... on PullRequestReview"`
	} `graphql:"node(id: $reviewId)"`
}

func (f *Fetcher) fetchPRGuidance(ctx context.Context, owner, repo string, prNumber int) ([]types.UserGuidance, error) {
	baseVars := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(prNumber),
	}

	var authorQuery prAuthorQuery
	if err := f.graphqlClient.Query(ctx, &authorQuery, baseVars); err != nil {
		return nil, err
	}
	prAuthor := authorQuery.Repository.PullRequest.Author.Login

	prComments, err := f.fetchAllPRComments(ctx, baseVars)
	if err != nil {
		return nil, err
	}

	reviews, err := f.fetchAllReviews(ctx, baseVars)
	if err != nil {
		return nil, err
	}

	authorizedUsers := map[string]bool{prAuthor: true}
	for _, review := range reviews {
		if review.State == "APPROVED" {
			assoc := review.AuthorAssociation
			if assoc == "OWNER" || assoc == "MEMBER" || assoc == "COLLABORATOR" {
				authorizedUsers[review.Author.Login] = true
			}
		}
	}

	var allGuidance []types.UserGuidance

	for _, comment := range prComments {
		guidanceContent, found := shared.ParseUserGuidance(comment.Body)
		if !found {
			continue
		}
		allGuidance = append(allGuidance, types.UserGuidance{
			Content:      guidanceContent,
			Author:       comment.Author.Login,
			Date:         comment.CreatedAt.Time,
			CommentURL:   comment.URL,
			IsAuthorized: authorizedUsers[comment.Author.Login],
		})
	}

	for _, review := range reviews {
		for _, comment := range review.Comments {
			guidanceContent, found := shared.ParseUserGuidance(comment.Body)
			if !found {
				continue
			}
			allGuidance = append(allGuidance, types.UserGuidance{
				Content:      guidanceContent,
				Author:       comment.Author.Login,
				Date:         comment.CreatedAt.Time,
				CommentURL:   comment.URL,
				IsAuthorized: authorizedUsers[comment.Author.Login],
			})
		}
	}

	return allGuidance, nil
}

type prComment struct {
	Body      string
	Author    struct{ Login string }
	CreatedAt githubv4.DateTime
	URL       string
}

type prReview struct {
	Author            struct{ Login string }
	State             string
	AuthorAssociation string
	Comments          []prComment
}

func (f *Fetcher) fetchAllPRComments(ctx context.Context, baseVars map[string]interface{}) ([]prComment, error) {
	var allComments []prComment
	var cursor *githubv4.String

	for {
		vars := map[string]interface{}{
			"owner":          baseVars["owner"],
			"repo":           baseVars["repo"],
			"number":         baseVars["number"],
			"commentsCursor": cursor,
		}

		var query prCommentsQuery
		if err := f.graphqlClient.Query(ctx, &query, vars); err != nil {
			return nil, err
		}

		for _, node := range query.Repository.PullRequest.Comments.Nodes {
			allComments = append(allComments, prComment{
				Body:      node.Body,
				Author:    node.Author,
				CreatedAt: node.CreatedAt,
				URL:       node.URL,
			})
		}

		if !query.Repository.PullRequest.Comments.PageInfo.HasNextPage {
			break
		}
		cursor = &query.Repository.PullRequest.Comments.PageInfo.EndCursor
	}

	return allComments, nil
}

func (f *Fetcher) fetchAllReviews(ctx context.Context, baseVars map[string]interface{}) ([]prReview, error) {
	var allReviews []prReview
	var cursor *githubv4.String

	for {
		vars := map[string]interface{}{
			"owner":         baseVars["owner"],
			"repo":          baseVars["repo"],
			"number":        baseVars["number"],
			"reviewsCursor": cursor,
		}

		var query prReviewsQuery
		if err := f.graphqlClient.Query(ctx, &query, vars); err != nil {
			return nil, err
		}

		for _, node := range query.Repository.PullRequest.Reviews.Nodes {
			review := prReview{
				Author:            node.Author,
				State:             node.State,
				AuthorAssociation: node.AuthorAssociation,
			}

			for _, c := range node.Comments.Nodes {
				review.Comments = append(review.Comments, prComment{
					Body:      c.Body,
					Author:    c.Author,
					CreatedAt: c.CreatedAt,
					URL:       c.URL,
				})
			}

			if node.Comments.PageInfo.HasNextPage {
				moreComments, err := f.fetchRemainingReviewComments(ctx, node.ID, node.Comments.PageInfo.EndCursor)
				if err != nil {
					return nil, err
				}
				review.Comments = append(review.Comments, moreComments...)
			}

			allReviews = append(allReviews, review)
		}

		if !query.Repository.PullRequest.Reviews.PageInfo.HasNextPage {
			break
		}
		cursor = &query.Repository.PullRequest.Reviews.PageInfo.EndCursor
	}

	return allReviews, nil
}

func (f *Fetcher) fetchRemainingReviewComments(ctx context.Context, reviewID githubv4.ID, startCursor githubv4.String) ([]prComment, error) {
	var allComments []prComment
	cursor := &startCursor

	for {
		vars := map[string]interface{}{
			"reviewId": reviewID,
			"cursor":   cursor,
		}

		var query reviewCommentsQuery
		if err := f.graphqlClient.Query(ctx, &query, vars); err != nil {
			return nil, err
		}

		for _, c := range query.Node.PullRequestReview.Comments.Nodes {
			allComments = append(allComments, prComment{
				Body:      c.Body,
				Author:    c.Author,
				CreatedAt: c.CreatedAt,
				URL:       c.URL,
			})
		}

		if !query.Node.PullRequestReview.Comments.PageInfo.HasNextPage {
			break
		}
		cursor = &query.Node.PullRequestReview.Comments.PageInfo.EndCursor
	}

	return allComments, nil
}
