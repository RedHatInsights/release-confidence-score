package shared

import (
	"regexp"
	"strings"
)

const (
	// Documentation file constants
	EntryPointFilename   = ".release-confidence-docs.md"
	AdditionalDocsHeader = "Additional Documentation"
)

// Package-level compiled regexes for performance
var (
	markdownLinkRegex          = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	plainURLRegex              = regexp.MustCompile(`https?://[^\s]+`)
	sectionHeaderRegex         = regexp.MustCompile(`(?m)^##\s+`)
	additionalDocsSectionRegex = regexp.MustCompile(`(?m)^##\s+` + regexp.QuoteMeta(AdditionalDocsHeader) + `\s*$`)
)

// ExtractDocumentationLinks finds markdown links in "Additional Documentation" section
// Returns a map of display name -> path
func ExtractDocumentationLinks(content string) map[string]string {
	// Extract content from "Additional Documentation" section only
	sectionContent := extractAdditionalDocsSection(content)
	if sectionContent == "" {
		return nil
	}

	links := make(map[string]string)

	// Extract markdown links: [display](path)
	for _, match := range markdownLinkRegex.FindAllStringSubmatch(sectionContent, -1) {
		if len(match) >= 3 {
			displayName := strings.TrimSpace(match[1])
			path := strings.TrimSpace(match[2])
			if displayName != "" && path != "" {
				links[displayName] = path
			}
		}
	}

	// Also extract plain URLs (as both display name and path)
	for _, url := range plainURLRegex.FindAllString(sectionContent, -1) {
		url = strings.TrimSpace(url)
		// Skip if already in markdown links
		found := false
		for _, path := range links {
			if path == url {
				found = true
				break
			}
		}
		if !found {
			links[url] = url
		}
	}

	return links
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

// ConvertToRawURL converts blob URLs to raw content URLs for both GitLab and GitHub
// GitLab: https://gitlab.com/owner/project/-/blob/main/file.md → /-/raw/main/file.md
// GitHub: https://github.com/owner/repo/blob/main/file.md → /raw/main/file.md
func ConvertToRawURL(url string) string {
	// GitLab format
	if strings.Contains(url, "/-/blob/") {
		return strings.Replace(url, "/-/blob/", "/-/raw/", 1)
	}
	// GitHub format
	if strings.Contains(url, "/blob/") {
		return strings.Replace(url, "/blob/", "/raw/", 1)
	}
	return url
}
