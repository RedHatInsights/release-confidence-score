package gitlab

import (
	"context"
	"encoding/base64"
	"fmt"

	"gitlab.com/gitlab-org/api/client-go"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// fetchDocumentation fetches complete repository documentation (README + linked docs)
func fetchDocumentation(client *gitlab.Client, host, projectPath string, cfg *config.Config) (*types.Documentation, error) {
	// Create repo client that implements types.RepositoryClient interface
	repoClient := newRepoClient(client, host, projectPath)

	// Use shared documentation fetcher (works for both GitHub and GitLab)
	sharedFetcher := shared.NewDocumentationFetcher(repoClient, cfg)

	// Fetch complete documentation
	return sharedFetcher.FetchCompleteDocsParsed(context.Background())
}

// repoClient implements types.RepositoryClient interface for GitLab
type repoClient struct {
	client      *gitlab.Client
	host        string
	projectPath string
	encodedPath string
	repoURL     string
}

// newRepoClient creates a new GitLab repository client
func newRepoClient(client *gitlab.Client, host, projectPath string) *repoClient {
	return &repoClient{
		client:      client,
		host:        host,
		projectPath: projectPath,
		encodedPath: urlEncodeProjectPath(projectPath),
		repoURL:     fmt.Sprintf("https://%s/%s", host, projectPath),
	}
}

// GetDefaultBranch returns the default branch name for the repository
func (r *repoClient) GetDefaultBranch(ctx context.Context) (string, error) {
	return getDefaultBranch(r.client, r.encodedPath)
}

// FetchFileContent fetches the content of a file from the repository
func (r *repoClient) FetchFileContent(ctx context.Context, path, ref string) (string, error) {
	return fetchFileContent(r.client, r.encodedPath, path, ref)
}

// GetRepositoryInfo returns the repository metadata
func (r *repoClient) GetRepositoryInfo() types.Repository {
	return types.Repository{
		Owner: extractOwnerFromProjectPath(r.projectPath),
		Name:  extractNameFromProjectPath(r.projectPath),
		URL:   r.repoURL,
		// DefaultBranch will be set by the shared fetcher
	}
}

// fetchFileContent fetches file content from a GitLab repository
func fetchFileContent(client *gitlab.Client, projectPath, filePath, ref string) (string, error) {
	file, _, err := client.RepositoryFiles.GetFile(projectPath, filePath, &gitlab.GetFileOptions{
		Ref: &ref,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch file %s: %w", filePath, err)
	}

	// Decode content if base64-encoded
	if file.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 content: %w", err)
		}
		return string(decoded), nil
	}

	return file.Content, nil
}

// getDefaultBranch gets the default branch of a GitLab project
func getDefaultBranch(client *gitlab.Client, projectPath string) (string, error) {
	project, _, err := client.Projects.GetProject(projectPath, &gitlab.GetProjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get project: %w", err)
	}

	if project.DefaultBranch == "" {
		return "main", nil // Fallback
	}

	return project.DefaultBranch, nil
}

// extractOwnerFromProjectPath extracts owner from GitLab project path
// For "group/repo" returns "group"
// For "group/subgroup/repo" returns "group/subgroup"
func extractOwnerFromProjectPath(projectPath string) string {
	lastSlash := -1
	for i := len(projectPath) - 1; i >= 0; i-- {
		if projectPath[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash == -1 {
		return ""
	}
	return projectPath[:lastSlash]
}

// extractNameFromProjectPath extracts repo name from GitLab project path
func extractNameFromProjectPath(projectPath string) string {
	lastSlash := -1
	for i := len(projectPath) - 1; i >= 0; i-- {
		if projectPath[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash == -1 {
		return projectPath
	}
	return projectPath[lastSlash+1:]
}
