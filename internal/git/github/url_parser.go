package github

import (
	"fmt"
	"regexp"
)

// githubCompareRegex matches GitHub compare URLs and extracts components
var githubCompareRegex = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/compare/([a-f0-9]+)\.\.\.([a-f0-9]+)$`)

// IsGitHubCompareURL checks if a URL is a GitHub compare URL
func IsGitHubCompareURL(url string) bool {
	return githubCompareRegex.MatchString(url)
}

// parseCompareURL extracts owner, repo, baseCommit, and headCommit from GitHub compare URL
func parseCompareURL(compareURL string) (owner, repo, baseCommit, headCommit string, err error) {
	// Parse: https://github.com/owner/repo/compare/sha1...sha2
	matches := githubCompareRegex.FindStringSubmatch(compareURL)
	if len(matches) != 5 {
		return "", "", "", "", fmt.Errorf("invalid GitHub compare URL format: %s", compareURL)
	}

	return matches[1], matches[2], matches[3], matches[4], nil
}
