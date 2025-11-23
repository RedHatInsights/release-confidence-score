package gitlab

import (
	"fmt"

	"gitlab.com/gitlab-org/api/client-go"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/shared"
)

func NewClient(cfg *config.Config) *gitlab.Client {
	var client *gitlab.Client
	var err error

	if cfg.GitLabSkipSSLVerify {
		httpClient := shared.NewHTTPClient(shared.HTTPClientOptions{
			SkipSSLVerify: true,
		})
		client, err = gitlab.NewClient(cfg.GitLabToken, gitlab.WithBaseURL(cfg.GitLabBaseURL), gitlab.WithHTTPClient(httpClient))
	} else {
		client, err = gitlab.NewClient(cfg.GitLabToken, gitlab.WithBaseURL(cfg.GitLabBaseURL))
	}

	if err != nil {
		panic(fmt.Sprintf("Failed to create GitLab client: %v", err))
	}

	return client
}

// PostMergeRequestComment posts a comment to a GitLab merge request
func PostMergeRequestComment(client *gitlab.Client, projectID string, mrIID int, body string) error {
	opts := &gitlab.CreateMergeRequestNoteOptions{
		Body: &body,
	}

	_, _, err := client.Notes.CreateMergeRequestNote(projectID, mrIID, opts)
	if err != nil {
		return fmt.Errorf("failed to create merge request note: %w", err)
	}

	return nil
}
