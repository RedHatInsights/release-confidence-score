package github

import (
	"log/slog"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/github/graphql"
	"release-confidence-score/internal/git/github/rest"
	"release-confidence-score/internal/git/types"
)

// NewProvider creates a GitHub provider based on configuration.
// Returns GraphQL-based provider if RCS_GITHUB_USE_GRAPHQL=true, otherwise REST-based.
func NewProvider(cfg *config.Config) types.GitProvider {
	if cfg.GitHubUseGraphQL {
		slog.Info("Using GitHub GraphQL API")
		return graphql.NewFetcher(cfg)
	}

	slog.Debug("Using GitHub REST API")
	return rest.NewFetcher(cfg)
}
