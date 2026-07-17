package github

import (
	"github.com/google/go-github/v89/github"
	"release-confidence-score/internal/config"
)

func NewClient(cfg *config.Config) (*github.Client, error) {
	return github.NewClient(github.WithAuthToken(cfg.GitHubToken))
}
