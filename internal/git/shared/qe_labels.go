package shared

import "strings"

// QE testing labels as they appear on GitHub PRs and GitLab MRs
const (
	LabelQETested       = "rcs/qe-tested"
	LabelNeedsQETesting = "rcs/needs-qe-testing"
)

// ExtractQELabel finds the QE testing label from a list of label names.
// Returns the matching label constant or empty string.
// If both labels are present, qe-tested takes precedence.
func ExtractQELabel(labelNames []string) string {
	hasQETested := false
	hasNeedsQETesting := false

	for _, name := range labelNames {
		switch strings.ToLower(name) {
		case LabelQETested:
			hasQETested = true
		case LabelNeedsQETesting:
			hasNeedsQETesting = true
		}
	}

	if hasQETested {
		return LabelQETested
	}
	if hasNeedsQETesting {
		return LabelNeedsQETesting
	}
	return ""
}
