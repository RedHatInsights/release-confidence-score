package github

import (
	"context"

	"github.com/google/go-github/v79/github"
	"release-confidence-score/internal/config"
)

func NewClient() *github.Client {
	cfg := config.Get()
	return github.NewTokenClient(context.Background(), cfg.GitHubToken)
}
