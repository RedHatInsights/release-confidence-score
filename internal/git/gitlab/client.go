package gitlab

import (
	"gitlab.com/gitlab-org/api/client-go"
	"release-confidence-score/internal/config"
	httputil "release-confidence-score/internal/http"
)

func NewClient(cfg *config.Config) (*gitlab.Client, error) {
	if cfg.GitLabSkipSSLVerify {
		httpClient := httputil.NewHTTPClient(httputil.HTTPClientOptions{
			SkipSSLVerify: true,
		})
		return gitlab.NewClient(cfg.GitLabToken, gitlab.WithBaseURL(cfg.GitLabBaseURL), gitlab.WithHTTPClient(httpClient))
	}

	return gitlab.NewClient(cfg.GitLabToken, gitlab.WithBaseURL(cfg.GitLabBaseURL))
}
