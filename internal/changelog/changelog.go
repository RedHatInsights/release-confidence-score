package changelog

import (
	"fmt"
	"strings"
)

// Changelog represents a changelog for a single repository
type Changelog struct {
	RepoURL string
	DiffURL string
	Commits []Commit
}

// Commit represents a single commit in the changelog
type Commit struct {
	SHA            string // Full commit SHA
	ShortSHA       string // Short SHA for display
	Message        string // Commit message (first line only)
	Author         string // Author name
	PRNumber       int    // Associated PR number (0 if none)
	QETestingLabel string // QE testing label status: "qe-tested", "needs-qe-testing", or empty
}

// FormatChangelog formats repository changelog data for display in the report as a table
func FormatChangelog(changelogs []*Changelog) string {
	if len(changelogs) == 0 {
		return "No repository changelog data available."
	}

	var result strings.Builder

	for i, changelog := range changelogs {
		// Add newline before each repository (except the first)
		if i > 0 {
			result.WriteString("\n\n")
		}

		result.WriteString(fmt.Sprintf("### [%s](%s)\n", changelog.RepoURL, changelog.DiffURL))

		if len(changelog.Commits) == 0 {
			result.WriteString("*No commits found in this comparison.*\n")
			continue
		}

		// Show commit count
		result.WriteString(fmt.Sprintf("*Total commits: %d*\n\n", len(changelog.Commits)))

		// Table header
		result.WriteString("| SHA | Message | Author | PR | QE Status |\n")
		result.WriteString("|-----|---------|--------|----|-----------|\n")

		// Table rows
		for _, commit := range changelog.Commits {
			// Create clickable SHA link
			shaLink := fmt.Sprintf("[%s](%s/commit/%s)", commit.ShortSHA, changelog.RepoURL, commit.SHA)

			// Escape pipe characters in commit message and author to prevent table breakage
			message := strings.ReplaceAll(commit.Message, "|", "\\|")
			author := strings.ReplaceAll(commit.Author, "|", "\\|")

			// PR link
			prLink := "N/A"
			if commit.PRNumber > 0 {
				prLink = fmt.Sprintf("[#%d](%s/pull/%d)", commit.PRNumber, changelog.RepoURL, commit.PRNumber)
			}

			// QE testing label status
			qeTesting := "N/A"
			switch commit.QETestingLabel {
			case "qe-tested":
				qeTesting = "✅ Tested"
			case "needs-qe-testing":
				qeTesting = "⚠️ Needs Testing"
			}

			result.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				shaLink, message, author, prLink, qeTesting))
		}
	}

	return result.String()
}
