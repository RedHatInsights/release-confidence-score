package shared

import (
	"fmt"
	"regexp"
	"strings"
)

// CompareURLRegex matches GitHub compare URLs and extracts components
var CompareURLRegex = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/compare/(.+?)\.\.\.([^?#]+)$`)

// ParseCompareURL extracts owner, repo, baseRef, and headRef from GitHub compare URL
func ParseCompareURL(compareURL string) (owner, repo, baseRef, headRef string, err error) {
	matches := CompareURLRegex.FindStringSubmatch(compareURL)
	if len(matches) != 5 {
		return "", "", "", "", fmt.Errorf("invalid GitHub compare URL format: %s", compareURL)
	}
	return matches[1], matches[2], matches[3], matches[4], nil
}

// ExtractRepoURL extracts the repository URL from a compare URL
func ExtractRepoURL(compareURL string) string {
	if idx := strings.Index(compareURL, "/compare/"); idx != -1 {
		return compareURL[:idx]
	}
	return compareURL
}
