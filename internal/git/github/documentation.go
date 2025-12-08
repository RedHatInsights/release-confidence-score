package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v79/github"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// fetchDocumentation fetches complete repository documentation (README + linked docs)
func fetchDocumentation(client *github.Client, owner, repo string, cfg *config.Config) (*types.Documentation, error) {
	// Create repo client that implements types.RepositoryClient interface
	repoClient := newRepoClient(client, owner, repo)

	// Use shared documentation fetcher (works for both GitHub and GitLab)
	sharedFetcher := shared.NewDocumentationFetcher(repoClient, cfg)

	// Fetch complete documentation
	return sharedFetcher.FetchCompleteDocsParsed(context.Background())
}

// repoClient implements types.RepositoryClient interface for GitHub
type repoClient struct {
	client  *github.Client
	owner   string
	repo    string
	repoURL string
}

// newRepoClient creates a new GitHub repository client
func newRepoClient(client *github.Client, owner, repo string) *repoClient {
	return &repoClient{
		client:  client,
		owner:   owner,
		repo:    repo,
		repoURL: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
	}
}

// GetDefaultBranch returns the default branch name for the repository
func (r *repoClient) GetDefaultBranch(ctx context.Context) (string, error) {
	repository, _, err := r.client.Repositories.Get(ctx, r.owner, r.repo)
	if err != nil || repository.DefaultBranch == nil {
		return "main", fmt.Errorf("failed to fetch default branch for %s/%s: %w", r.owner, r.repo, err)
	}
	return *repository.DefaultBranch, nil
}

// FetchFileContent fetches the content of a file from the repository
func (r *repoClient) FetchFileContent(ctx context.Context, path, ref string) (string, error) {
	// Use GitHub SDK to fetch file content
	fileContent, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.repo, path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s from %s/%s: %w", path, r.owner, r.repo, err)
	}

	// GitHub SDK automatically handles base64 decoding
	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode content for %s from %s/%s: %w", path, r.owner, r.repo, err)
	}

	return content, nil
}

// GetRepositoryInfo returns the repository metadata
func (r *repoClient) GetRepositoryInfo() types.Repository {
	return types.Repository{
		Owner: r.owner,
		Name:  r.repo,
		URL:   r.repoURL,
		// DefaultBranch will be set by the shared fetcher
	}
}
