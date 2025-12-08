package shared

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/types"
	httputil "release-confidence-score/internal/http"
)

const (
	httpTimeout = 30 * time.Second
)

// DocumentationFetcher provides shared documentation fetching logic
type DocumentationFetcher struct {
	repoClient types.RepositoryClient
	config     *config.Config
}

// NewDocumentationFetcher creates a new documentation fetcher
func NewDocumentationFetcher(repoClient types.RepositoryClient, cfg *config.Config) *DocumentationFetcher {
	return &DocumentationFetcher{
		repoClient: repoClient,
		config:     cfg,
	}
}

// FetchCompleteDocsParsed fetches entry point documentation and all linked docs
func (d *DocumentationFetcher) FetchCompleteDocsParsed(ctx context.Context) (*types.Documentation, error) {
	// Get repository info
	repoInfo := d.repoClient.GetRepositoryInfo()

	// Fetch default branch
	defaultBranch, err := d.repoClient.GetDefaultBranch(ctx)
	if err != nil {
		slog.Debug("Failed to get default branch", "repo", repoInfo.URL, "error", err)
		defaultBranch = "main"
	}

	// Update repo info with default branch
	repoInfo.DefaultBranch = defaultBranch

	// Try to fetch entry point document
	entryDoc, err := d.repoClient.FetchFileContent(ctx, EntryPointFilename, defaultBranch)
	if err != nil {
		slog.Debug("No entry point documentation found", "repo", repoInfo.URL, "error", err)
		return newEmptyDocumentation(repoInfo), nil
	}

	// Parse and extract links from "Additional Documentation" section
	linkedPaths := ExtractDocumentationLinks(entryDoc)

	// Fetch all linked documents
	linkedDocs, linkedDocsOrder := d.fetchLinkedDocs(ctx, defaultBranch, linkedPaths)

	return &types.Documentation{
		MainDocContent:  entryDoc,
		MainDocFile:     EntryPointFilename,
		LinkedDocs:      linkedDocs,
		LinkedDocsOrder: linkedDocsOrder,
		Repository:      repoInfo,
	}, nil
}

// fetchLinkedDocs fetches all linked documentation files
func (d *DocumentationFetcher) fetchLinkedDocs(ctx context.Context, ref string, linkedPaths map[string]string) (map[string]string, []string) {
	linkedDocs := make(map[string]string)
	linkedDocsOrder := []string{}

	for displayName, path := range linkedPaths {
		content, err := d.fetchDocumentationFile(ctx, ref, path)
		if err != nil {
			slog.Warn("Failed to fetch linked documentation",
				"path", path,
				"error", err)
			continue
		}

		linkedDocs[displayName] = content
		linkedDocsOrder = append(linkedDocsOrder, displayName)
		slog.Debug("Fetched linked documentation",
			"display_name", displayName,
			"size", len(content))
	}

	return linkedDocs, linkedDocsOrder
}

// fetchDocumentationFile fetches a documentation file from repository or external URL
func (d *DocumentationFetcher) fetchDocumentationFile(ctx context.Context, ref, path string) (string, error) {
	// Check if it's an external URL
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// Convert blob URLs to raw URLs
		rawURL := ConvertToRawURL(path)
		return d.fetchExternalURL(ctx, rawURL)
	}

	// Fetch from repository
	content, err := d.repoClient.FetchFileContent(ctx, path, ref)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file %s: %w", path, err)
	}

	return content, nil
}

// fetchExternalURL fetches content from an external URL
func (d *DocumentationFetcher) fetchExternalURL(ctx context.Context, url string) (string, error) {
	// Determine if this is a GitLab URL for SSL verification settings
	isGitLab := strings.Contains(url, "gitlab")
	skipSSLVerify := isGitLab && d.config.GitLabSkipSSLVerify

	httpClient := httputil.NewHTTPClient(httputil.HTTPClientOptions{
		Timeout:       httpTimeout,
		SkipSSLVerify: skipSSLVerify,
	})

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitLab authentication if needed
	if isGitLab && d.config.GitLabToken != "" {
		req.Header.Set("PRIVATE-TOKEN", d.config.GitLabToken)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// newEmptyDocumentation creates an empty documentation object
func newEmptyDocumentation(repo types.Repository) *types.Documentation {
	return &types.Documentation{
		MainDocContent:  "",
		MainDocFile:     "",
		LinkedDocs:      make(map[string]string),
		LinkedDocsOrder: []string{},
		Repository:      repo,
	}
}
