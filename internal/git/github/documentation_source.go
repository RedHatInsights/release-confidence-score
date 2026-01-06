package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v81/github"
)

// documentationSource implements DocumentationSource interface for GitHub
type documentationSource struct {
	client *github.Client
	owner  string
	repo   string
}

// newDocumentationSource creates a new GitHub documentation source
func newDocumentationSource(client *github.Client, owner, repo string) *documentationSource {
	return &documentationSource{
		client: client,
		owner:  owner,
		repo:   repo,
	}
}

// GetDefaultBranch returns the default branch name for the repository
func (d *documentationSource) GetDefaultBranch(ctx context.Context) (string, error) {
	repository, _, err := d.client.Repositories.Get(ctx, d.owner, d.repo)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repository info for %s/%s: %w", d.owner, d.repo, err)
	}

	// Empty repositories have nil DefaultBranch - use "main" as default
	if repository.DefaultBranch == nil || *repository.DefaultBranch == "" {
		return "main", nil
	}

	return *repository.DefaultBranch, nil
}

// FetchFileContent fetches the content of a file from the repository
func (d *documentationSource) FetchFileContent(ctx context.Context, path, ref string) (string, error) {
	// Use GitHub SDK to fetch file content
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	fileContent, _, _, err := d.client.Repositories.GetContents(ctx, d.owner, d.repo, path, opts)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s from %s/%s: %w", path, d.owner, d.repo, err)
	}

	// GitHub SDK automatically handles base64 decoding
	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode content for %s from %s/%s: %w", path, d.owner, d.repo, err)
	}

	return content, nil
}
