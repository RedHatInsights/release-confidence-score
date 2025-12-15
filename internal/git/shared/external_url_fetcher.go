package shared

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	httputil "release-confidence-score/internal/http"
)

const httpTimeout = 30 * time.Second

// fetchExternalURL fetches content from an external URL
func (d *DocumentationFetcher) fetchExternalURL(ctx context.Context, urlStr string) (string, error) {
	// Determine if this is a GitLab URL for SSL verification settings
	isGitLab := isGitLabURL(urlStr)
	skipSSLVerify := isGitLab && d.config.GitLabSkipSSLVerify

	httpClient := httputil.NewHTTPClient(httputil.HTTPClientOptions{
		Timeout:       httpTimeout,
		SkipSSLVerify: skipSSLVerify,
	})

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
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

// isGitLabURL checks if a URL is a GitLab URL by parsing and examining the hostname
func isGitLabURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// Fallback to simple string matching if parsing fails
		return strings.Contains(urlStr, "gitlab")
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	// Check for gitlab.com or self-hosted GitLab instances (gitlab.*)
	return hostname == "gitlab.com" ||
		hostname == "www.gitlab.com" ||
		strings.HasPrefix(hostname, "gitlab.")
}
