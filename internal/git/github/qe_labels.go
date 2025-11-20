package github

import (
	"github.com/google/go-github/v79/github"
)

// GetPRQELabel extracts the QE testing label from a PR object
// Returns "qe-tested", "needs-qe-testing", or empty string
func GetPRQELabel(pr *github.PullRequest) string {
	if pr == nil {
		return ""
	}

	return findQETestingLabels(pr.Labels)
}

// findQETestingLabels extracts QE testing label from PR labels with priority logic
func findQETestingLabels(labels []*github.Label) string {
	hasQeTested := false
	hasNeedsQETesting := false

	for _, label := range labels {
		labelName := label.GetName()
		if labelName == "rcs/qe-tested" {
			hasQeTested = true
		} else if labelName == "rcs/needs-qe-testing" {
			hasNeedsQETesting = true
		}
	}

	// Priority logic: qe-tested takes precedence over needs-qe-testing
	if hasQeTested {
		return "qe-tested"
	} else if hasNeedsQETesting {
		return "needs-qe-testing"
	}
	return ""
}
