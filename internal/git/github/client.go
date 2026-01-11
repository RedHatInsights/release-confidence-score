package github

import (
	"github.com/google/go-github/v81/github"
	"release-confidence-score/internal/config"
)

func NewClient(cfg *config.Config) *github.Client {
	return github.NewClient(nil).WithAuthToken(cfg.GitHubToken)
}
