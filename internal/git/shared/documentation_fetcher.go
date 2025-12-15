package shared

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/types"
)

const (
	// Documentation file constants
	mainDocFilename      = ".release-confidence-docs.md"
	additionalDocsHeader = "Additional Documentation"
)

// Package-level compiled regexes for performance
var (
	markdownLinkRegex          = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	plainURLRegex              = regexp.MustCompile(`https?://[^\s]+`)
	sectionHeaderRegex         = regexp.MustCompile(`(?m)^##\s+`)
	additionalDocsSectionRegex = regexp.MustCompile(`(?m)^##\s+` + regexp.QuoteMeta(additionalDocsHeader) + `\s*$`)
)

// DocumentationFetcher provides shared documentation fetching logic
type DocumentationFetcher struct {
	source         types.DocumentationSource
	config         *config.Config
	baseRepository types.Repository
}

// NewDocumentationFetcher creates a new documentation fetcher
func NewDocumentationFetcher(source types.DocumentationSource, baseRepo types.Repository, cfg *config.Config) *DocumentationFetcher {
	return &DocumentationFetcher{
		source:         source,
		config:         cfg,
		baseRepository: baseRepo,
	}
}

// FetchAllDocs fetches entry point documentation and all additional docs
func (d *DocumentationFetcher) FetchAllDocs(ctx context.Context) (*types.Documentation, error) {
	// Fetch default branch
	defaultBranch, err := d.source.GetDefaultBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}

	// Build complete repository info with default branch
	repository := d.baseRepository
	repository.DefaultBranch = defaultBranch

	// Try to fetch main documentation
	mainDocContent, err := d.source.FetchFileContent(ctx, mainDocFilename, defaultBranch)
	if err != nil {
		slog.Debug("No main documentation file found", "repo", repository.URL, "error", err)
		return &types.Documentation{
			Repository: repository,
		}, nil
	}

	// Parse and extract paths from "Additional Documentation" section
	additionalDocPaths, additionalDocsOrder := extractAdditionalDocPaths(mainDocContent)

	// Fetch additional docs
	additionalDocsContent, failedAdditionalDocs := d.fetchAdditionalDocs(ctx, defaultBranch, additionalDocPaths, additionalDocsOrder)

	return &types.Documentation{
		MainDocContent:        mainDocContent,
		MainDocFile:           mainDocFilename,
		AdditionalDocsContent: additionalDocsContent,
		AdditionalDocsOrder:   additionalDocsOrder,
		FailedAdditionalDocs:  failedAdditionalDocs,
		Repository:            repository,
	}, nil
}

// extractAdditionalDocPaths finds markdown links in "Additional Documentation" section
// Returns: map of display name -> path, and order as they appear in the file
func extractAdditionalDocPaths(content string) (map[string]string, []string) {
	// Extract content from "Additional Documentation" section only
	sectionContent := extractAdditionalDocSection(content)
	if sectionContent == "" {
		return nil, nil
	}

	paths := make(map[string]string)
	order := []string{}
	seenPaths := make(map[string]bool)

	// Extract markdown links: [display](path) - preserving file order
	for _, match := range markdownLinkRegex.FindAllStringSubmatch(sectionContent, -1) {
		if len(match) >= 3 {
			displayName := strings.TrimSpace(match[1])
			path := strings.TrimSpace(match[2])
			if displayName != "" && path != "" {
				paths[displayName] = path
				order = append(order, displayName)
				seenPaths[path] = true
			}
		}
	}

	// Also extract plain URLs (as both display name and path) - preserving file order
	for _, url := range plainURLRegex.FindAllString(sectionContent, -1) {
		url = strings.TrimSpace(url)
		// Skip if already in markdown links (O(1) lookup instead of O(n))
		if !seenPaths[url] {
			paths[url] = url
			order = append(order, url)
			seenPaths[url] = true
		}
	}

	return paths, order
}

// extractAdditionalDocSection extracts the "Additional Documentation" section using pre-compiled regex
func extractAdditionalDocSection(content string) string {
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

// fetchAdditionalDocs fetches all additional documentation files
// Iterates in file order but doesn't filter the order list (template handles missing docs)
// Returns: successfully fetched docs, failed docs (display name -> error message)
func (d *DocumentationFetcher) fetchAdditionalDocs(ctx context.Context, ref string, additionalDocPaths map[string]string, additionalDocsOrder []string) (map[string]string, map[string]string) {
	additionalDocs := make(map[string]string)
	failedDocs := make(map[string]string)

	// Fetch documents in file order
	for _, displayName := range additionalDocsOrder {
		path := additionalDocPaths[displayName]
		content, err := d.fetchAdditionalDocContent(ctx, ref, path)
		if err != nil {
			// Track failure for reporting, but don't fail the entire operation
			failedDocs[displayName] = err.Error()
			slog.Warn("Failed to fetch additional documentation",
				"path", path,
				"error", err)
			continue
		}

		additionalDocs[displayName] = content
		slog.Debug("Fetched additional documentation",
			"display_name", displayName,
			"size", len(content))
	}

	return additionalDocs, failedDocs
}

// fetchAdditionalDocContent fetches a documentation file from repository or external URL
func (d *DocumentationFetcher) fetchAdditionalDocContent(ctx context.Context, ref, path string) (string, error) {
	// Check if it's an external URL
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// Convert blob URLs to raw URLs (both GitHub /blob/ and GitLab /-/blob/)
		rawURL := strings.Replace(path, "/blob/", "/raw/", 1)
		return d.fetchExternalURL(ctx, rawURL)
	}

	// Fetch from repository
	content, err := d.source.FetchFileContent(ctx, path, ref)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file %s: %w", path, err)
	}

	return content, nil
}
