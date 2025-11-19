package shared

import (
	"regexp"
	"time"
)

// UserGuidance represents a complete user guidance with metadata for reporting
type UserGuidance struct {
	Content      string    // The actual guidance content
	Author       string    // GitHub username who posted it
	Date         time.Time // When it was posted
	CommentURL   string    // Direct link to the GitHub comment
	IsAuthorized bool      // Whether the author had permission to post
}

// Pre-compiled regex for user guidance parsing.
// Matches "/rcs <guidance text>" where guidance text can span multiple lines (case-insensitive).
var guidancePattern = regexp.MustCompile(`(?is)/rcs\s+(\S.*\S|\S)`)

// ParseUserGuidance extracts user guidance from text using a regex pattern.
// If the text starts with /rcs, everything after it is captured as guidance.
// Returns the guidance text and true if found, empty string and false otherwise.
func ParseUserGuidance(text string) (string, bool) {
	matches := guidancePattern.FindStringSubmatch(text)
	if len(matches) > 1 {
		guidance := matches[1]
		return guidance, true
	}

	return "", false
}
