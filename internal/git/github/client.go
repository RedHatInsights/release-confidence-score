package github

import (
	"context"

	"github.com/google/go-github/v79/github"
	"release-confidence-score/internal/config"
)

func NewClient(cfg *config.Config) *github.Client {
	return github.NewTokenClient(context.Background(), cfg.GitHubToken)
}
