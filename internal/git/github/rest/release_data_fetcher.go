package rest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v80/github"
	"golang.org/x/sync/errgroup"
	"release-confidence-score/internal/config"
	ghshared "release-confidence-score/internal/git/github/shared"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// Fetcher implements the GitProvider interface using GitHub REST API
type Fetcher struct {
	client *github.Client
	config *config.Config
}

// NewFetcher creates a new GitHub REST-based fetcher
func NewFetcher(cfg *config.Config) *Fetcher {
	return &Fetcher{
		client: ghshared.NewRESTClient(cfg.GitHubToken),
		config: cfg,
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
	slog.Debug("Fetching GitHub release data via REST", "url", compareURL)

	owner, repo, baseRef, headRef, err := ghshared.ParseCompareURL(compareURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse GitHub compare URL: %w", err)
	}

	slog.Debug("Parsed compare URL", "owner", owner, "repo", repo, "base", baseRef, "head", headRef)

	cache := newPRCache()

	g, gCtx := errgroup.WithContext(ctx)

	var comparison *types.Comparison
	var userGuidance []types.UserGuidance
	var documentation *types.Documentation

	// Fetch diff and user guidance
	g.Go(func() error {
		var err error
		comparison, err = fetchDiff(gCtx, f.client, owner, repo, baseRef, headRef, compareURL, cache)
		if err != nil {
			return fmt.Errorf("failed to fetch comparison: %w", err)
		}

		userGuidance, err = fetchUserGuidance(gCtx, f.client, owner, repo, comparison, cache)
		if err != nil {
			return fmt.Errorf("failed to fetch user guidance: %w", err)
		}
		return nil
	})

	// Fetch documentation (independent)
	g.Go(func() error {
		docSource := ghshared.NewDocumentationSource(f.client, owner, repo)
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

	slog.Debug("Release data fetched successfully via REST",
		"commit_entries", len(comparison.Commits),
		"user_guidance_items", len(userGuidance),
		"files", len(comparison.Files),
		"has_documentation", documentation != nil)

	return comparison, userGuidance, documentation, nil
}
