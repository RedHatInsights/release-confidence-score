package gitlab

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// documentationSource implements DocumentationSource interface for GitLab
type documentationSource struct {
	client      *gitlab.Client
	host        string
	projectPath string
}

// newDocumentationSource creates a new GitLab documentation source
func newDocumentationSource(client *gitlab.Client, host, projectPath string) *documentationSource {
	return &documentationSource{
		client:      client,
		host:        host,
		projectPath: projectPath,
	}
}

// GetDefaultBranch returns the default branch name for the repository
func (d *documentationSource) GetDefaultBranch(ctx context.Context) (string, error) {
	project, _, err := d.client.Projects.GetProject(url.PathEscape(d.projectPath), &gitlab.GetProjectOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to fetch repository info for %s: %w", d.projectPath, err)
	}

	// Empty repositories have empty DefaultBranch - use "main" as default
	if project.DefaultBranch == "" {
		return "main", nil
	}

	return project.DefaultBranch, nil
}

// FetchFileContent fetches the content of a file from the repository
func (d *documentationSource) FetchFileContent(ctx context.Context, path, ref string) (string, error) {
	// Use GitLab SDK to fetch file content
	opts := &gitlab.GetFileOptions{Ref: &ref}
	file, _, err := d.client.RepositoryFiles.GetFile(url.PathEscape(d.projectPath), path, opts, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s from %s: %w", path, d.projectPath, err)
	}

	// Decode content if base64-encoded
	if file.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 content for %s from %s: %w", path, d.projectPath, err)
		}
		return string(decoded), nil
	}

	return file.Content, nil
}
