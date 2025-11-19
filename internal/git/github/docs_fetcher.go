package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/shared"

	"github.com/google/go-github/v76/github"
)

const (
	// Documentation file constants
	entryPointFilename   = ".release-confidence-docs.md"
	additionalDocsHeader = "Additional Documentation"

	// HTTP client timeout for external documentation fetching
	httpTimeout = 30 * time.Second
)

// Package-level compiled regexes for performance
var (
	markdownLinkRegex  = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	plainURLRegex      = regexp.MustCompile(`https?://[^\s]+`)
	sectionHeaderRegex = regexp.MustCompile(`(?m)^##\s+`)
	// Pre-compile regex for "Additional Documentation" section (most common use case)
	additionalDocsSectionRegex = regexp.MustCompile(`(?m)^##\s+` + regexp.QuoteMeta(additionalDocsHeader) + `\s*$`)
)

var (
	errEmptyRepoOwner = errors.New("repository owner cannot be empty")
	errEmptyRepoName  = errors.New("repository name cannot be empty")
)

type DocumentationFetcher struct {
	client *github.Client
}

type RepoDocumentation struct {
	EntryPoint      string            // Content of .release-confidence-docs.md
	EntryPointFile  string            // Which file was used as entry point
	LinkedDocs      map[string]string // filename -> content
	LinkedDocsOrder []string          // preserves original order of linked docs
	TotalSize       int               // Track content size
	RepoURL         string            // Source repository URL
	DefaultBranch   string            // Default branch name (e.g., "main", "master")
}

func NewDocumentationFetcher(client *github.Client) *DocumentationFetcher {
	return &DocumentationFetcher{client: client}
}

// FetchCompleteDocsParsed fetches entry point documentation and all linked docs
// Context allows caller to control timeouts and cancellation
func (d *DocumentationFetcher) FetchCompleteDocsParsed(ctx context.Context, repoOwner, repoName string) (*RepoDocumentation, error) {
	// Validate inputs
	if repoOwner == "" {
		return nil, errEmptyRepoOwner
	}
	if repoName == "" {
		return nil, errEmptyRepoName
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)

	// Fetch repository metadata to get default branch
	defaultBranch := d.getDefaultBranch(ctx, repoOwner, repoName)

	// Try to fetch entry point document
	entryDoc, entryPointFile, err := d.fetchEntryPointDoc(ctx, repoOwner, repoName)
	if err != nil {
		// No documentation found - return empty documentation (not an error)
		slog.Debug("No entry point documentation found",
			"repo", repoURL,
			"error", err)
		return newEmptyDocumentation(repoURL, defaultBranch), nil
	}

	// Parse and extract relative links from "Additional Documentation" section
	linkedPaths := extractDocumentationLinks(entryDoc)

	// Fetch all linked documents (preserving order)
	linkedDocs, linkedDocsOrder := d.fetchLinkedDocs(ctx, repoOwner, repoName, linkedPaths)

	// Calculate total size
	totalSize := len(entryDoc)
	for _, content := range linkedDocs {
		totalSize += len(content)
	}

	return &RepoDocumentation{
		EntryPoint:      entryDoc,
		EntryPointFile:  entryPointFile,
		LinkedDocs:      linkedDocs,
		LinkedDocsOrder: linkedDocsOrder,
		TotalSize:       totalSize,
		RepoURL:         repoURL,
		DefaultBranch:   defaultBranch,
	}, nil
}

// newEmptyDocumentation creates a documentation struct for repos without docs
func newEmptyDocumentation(repoURL, defaultBranch string) *RepoDocumentation {
	return &RepoDocumentation{
		EntryPoint:      "",
		EntryPointFile:  "",
		LinkedDocs:      make(map[string]string),
		LinkedDocsOrder: []string{},
		TotalSize:       0,
		RepoURL:         repoURL,
		DefaultBranch:   defaultBranch,
	}
}

// getDefaultBranch fetches the repository's default branch name
func (d *DocumentationFetcher) getDefaultBranch(ctx context.Context, owner, repo string) string {
	repository, _, err := d.client.Repositories.Get(ctx, owner, repo)
	if err != nil || repository.DefaultBranch == nil {
		slog.Debug("Failed to fetch default branch, using 'main'",
			"owner", owner,
			"repo", repo,
			"error", err)
		return "main" // Fallback to "main" if we can't fetch the default branch
	}
	return *repository.DefaultBranch
}

// fetchEntryPointDoc tries to fetch .release-confidence-docs.md only
func (d *DocumentationFetcher) fetchEntryPointDoc(ctx context.Context, owner, repo string) (string, string, error) {
	content, err := FetchFileContent(d.client, owner, repo, entryPointFilename)
	if err != nil {
		return "", "", fmt.Errorf("no %s file found in %s/%s: %w", entryPointFilename, owner, repo, err)
	}

	return content, entryPointFilename, nil
}

// extractDocumentationLinks finds markdown links in "Additional Documentation" section
func extractDocumentationLinks(content string) []string {
	// Extract content from "Additional Documentation" section only
	// Use pre-compiled regex for this common case
	sectionContent := extractAdditionalDocsSection(content)
	if sectionContent == "" {
		return []string{}
	}

	var paths []string
	seen := make(map[string]bool)

	// Extract markdown links: [text](path)
	for _, match := range markdownLinkRegex.FindAllStringSubmatch(sectionContent, -1) {
		if len(match) > 2 {
			addMarkdownPath(match[2], &paths, seen)
		}
	}

	// Extract plain URLs (https://... or http://...)
	for _, url := range plainURLRegex.FindAllString(sectionContent, -1) {
		addMarkdownPath(url, &paths, seen)
	}

	return paths
}

// extractAdditionalDocsSection extracts the "Additional Documentation" section using pre-compiled regex
func extractAdditionalDocsSection(content string) string {
	// Find the section start
	sectionMatch := additionalDocsSectionRegex.FindStringIndex(content)
	if sectionMatch == nil {
		return "" // Section not found
	}

	// Start after the section header
	startIndex := sectionMatch[1]

	// Find the next same-level section (##) or end of document
	remainingContent := content[startIndex:]
	nextSectionMatch := sectionHeaderRegex.FindStringIndex(remainingContent)

	var endIndex int
	if nextSectionMatch != nil {
		// Found next section - content ends before it
		endIndex = startIndex + nextSectionMatch[0]
	} else {
		// No next section found - content goes to end of document
		endIndex = len(content)
	}

	return strings.TrimSpace(content[startIndex:endIndex])
}

// addMarkdownPath adds a path to the list if it's a .md file and not a duplicate
func addMarkdownPath(path string, paths *[]string, seen map[string]bool) {
	// Only process .md files
	if !strings.HasSuffix(path, ".md") {
		return
	}

	// Clean up relative path references
	if !strings.HasPrefix(path, "http") {
		path = strings.TrimPrefix(path, "./")
	}

	// Avoid duplicates
	if !seen[path] {
		*paths = append(*paths, path)
		seen[path] = true
	}
}

// fetchLinkedDocs fetches all linked documents from the Additional Documentation section
// Returns both the content map and the original order slice
func (d *DocumentationFetcher) fetchLinkedDocs(ctx context.Context, owner, repo string, paths []string) (map[string]string, []string) {
	docs := make(map[string]string)
	var order []string

	for _, path := range paths {
		var content string
		var err error

		if strings.HasPrefix(path, "http") {
			// External URL - fetch via HTTP
			content, err = d.fetchExternalURL(ctx, path)
		} else {
			// Local file - fetch from repository
			content, err = FetchFileContent(d.client, owner, repo, path)
		}

		if err != nil {
			slog.Debug("Failed to fetch linked documentation",
				"repo", fmt.Sprintf("%s/%s", owner, repo),
				"path", path,
				"error", err)
			continue
		}

		docs[path] = content
		order = append(order, path)
	}

	return docs, order
}

// fetchExternalURL fetches content from an external URL with timeout and proper error handling
func (d *DocumentationFetcher) fetchExternalURL(ctx context.Context, url string) (string, error) {
	// Convert GitLab/GitHub blob URLs to raw URLs
	rawURL, err := convertToRawURL(url)
	if err != nil {
		return "", fmt.Errorf("failed to convert URL to raw format: %w", err)
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for %s: %w", rawURL, err)
	}

	// Configure GitLab-specific settings if needed
	cfg := config.Get()
	client := createHTTPClient(rawURL, cfg)
	configureGitLabRequest(req, rawURL, cfg)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch external URL %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch external URL %s: HTTP %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body from %s: %w", rawURL, err)
	}

	return string(body), nil
}

// createHTTPClient creates an HTTP client with timeout and appropriate settings for the URL
func createHTTPClient(url string, cfg *config.Config) *http.Client {
	isGitLab := strings.Contains(url, "gitlab")
	skipSSLVerify := isGitLab && cfg.GitLabSkipSSLVerify

	return shared.NewHTTPClient(shared.HTTPClientOptions{
		Timeout:       httpTimeout,
		SkipSSLVerify: skipSSLVerify,
	})
}

// configureGitLabRequest adds GitLab-specific authentication headers
func configureGitLabRequest(req *http.Request, url string, cfg *config.Config) {
	if strings.Contains(url, "gitlab") && cfg.GitLabToken != "" {
		req.Header.Set("PRIVATE-TOKEN", cfg.GitLabToken)
	}
}

// convertToRawURL converts blob URLs to raw content URLs for both GitLab and GitHub
// Returns an error if the URL doesn't match expected patterns
func convertToRawURL(url string) (string, error) {
	// Only convert URLs from known Git hosting platforms
	if !strings.Contains(url, "github") && !strings.Contains(url, "gitlab") {
		return url, nil
	}

	// Both GitLab and GitHub use /blob/ in their URLs:
	// GitLab: https://gitlab.example.com/group/project/-/blob/branch/file.md → /-/raw/
	// GitHub: https://github.com/owner/repo/blob/branch/file.md → /raw/
	if !strings.Contains(url, "/blob/") {
		return "", fmt.Errorf("expected /blob/ in git hosting URL but not found: %s", url)
	}

	return strings.Replace(url, "/blob/", "/raw/", 1), nil
}

// FormatForLLM combines all documentation into LLM-friendly format
// Returns empty string if no documentation is found to avoid sending empty sections to LLM
// shiftMarkdownHeaders increases all markdown header levels by the specified amount
// This prevents header level conflicts between prompt structure and documentation content
func shiftMarkdownHeaders(content string, levels int) string {
	if levels <= 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Count existing header level
			headerLevel := 0
			for _, char := range trimmed {
				if char == '#' {
					headerLevel++
				} else {
					break
				}
			}

			// Only process if it's a valid header (has space after #'s)
			if headerLevel > 0 && len(trimmed) > headerLevel && trimmed[headerLevel] == ' ' {
				// Shift header level
				newLevel := headerLevel + levels
				newHeader := strings.Repeat("#", newLevel) + trimmed[headerLevel:]
				lines[i] = strings.Replace(line, trimmed, newHeader, 1)
			}
		}
	}

	return strings.Join(lines, "\n")
}

// FormatDocumentationForLLM formats repository documentation for LLM consumption
func FormatDocumentationForLLM(docs *RepoDocumentation) string {
	// If no documentation found, return empty string to omit from user prompt
	if docs.EntryPointFile == "" {
		return ""
	}

	var result strings.Builder

	result.WriteString("=== REPOSITORY DOCUMENTATION ===\n")
	result.WriteString(fmt.Sprintf("Repository: %s\n", docs.RepoURL))
	result.WriteString(fmt.Sprintf("Documentation found: %s\n", docs.EntryPointFile))

	if len(docs.LinkedDocs) > 0 {
		result.WriteString(fmt.Sprintf("Additional docs: %d linked files\n", len(docs.LinkedDocs)))
	}
	result.WriteString("\n")

	// Primary documentation - shift headers by 2 levels to nest under prompt structure
	// User prompt uses ##, this section uses ###, so doc content should start at ####
	result.WriteString("### Primary Documentation\n")
	result.WriteString(shiftMarkdownHeaders(docs.EntryPoint, 3))
	result.WriteString("\n\n")

	// Linked documents (if any) - preserve original order
	if len(docs.LinkedDocs) > 0 {
		result.WriteString(fmt.Sprintf("### %s\n\n", additionalDocsHeader))

		for _, filename := range docs.LinkedDocsOrder {
			if content, exists := docs.LinkedDocs[filename]; exists {
				result.WriteString(fmt.Sprintf("#### %s\n", filename))
				result.WriteString(shiftMarkdownHeaders(content, 4))
				result.WriteString("\n\n")
			}
		}
	}

	result.WriteString(fmt.Sprintf("*Total documentation size: %d characters*\n", docs.TotalSize))

	return result.String()
}

// FormatMultipleRepoDocumentationForLLM formats multiple repository documentation sets for LLM consumption
func FormatMultipleRepoDocumentationForLLM(documentation []*RepoDocumentation) string {
	if len(documentation) == 0 {
		return ""
	}

	var result strings.Builder
	for i, docs := range documentation {
		if i > 0 {
			result.WriteString("\n\n")
		}
		formatted := FormatDocumentationForLLM(docs)
		if formatted != "" {
			result.WriteString(formatted)
		}
	}

	return result.String()
}
