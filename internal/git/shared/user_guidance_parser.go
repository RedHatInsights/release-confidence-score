package shared

import (
	"regexp"
)

// Pre-compiled regex for user guidance parsing.
// Matches "/rcs <guidance text>" at the start of text (after optional leading whitespace).
// Guidance text can span multiple lines (case-insensitive).
var guidancePattern = regexp.MustCompile(`(?is)^\s*/rcs\s+(\S.*\S|\S)`)

// ParseUserGuidance extracts user guidance from text using a regex pattern.
// If the text contains /rcs, everything after it is captured as guidance.
// Returns the guidance text and true if found, empty string and false otherwise.
func ParseUserGuidance(text string) (string, bool) {
	matches := guidancePattern.FindStringSubmatch(text)
	if len(matches) > 1 {
		guidance := matches[1]
		return guidance, true
	}

	return "", false
}
