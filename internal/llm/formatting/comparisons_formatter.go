package formatting

import (
	"fmt"
	"strings"

	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
)

// FormatComparisons formats multiple comparisons for LLM consumption
func FormatComparisons(comparisons []*types.Comparison) string {
	if len(comparisons) == 0 {
		return ""
	}

	var result strings.Builder
	for i, comparison := range comparisons {
		if comparison == nil || len(comparison.Commits) == 0 {
			continue
		}

		if len(comparisons) > 1 {
			result.WriteString(fmt.Sprintf("=== Diff %d: %s ===\n\n", i+1, comparison.RepoURL))
		} else {
			result.WriteString(fmt.Sprintf("Repository: %s\n\n", comparison.RepoURL))
		}

		result.WriteString("Commits:\n")
		for _, commit := range comparison.Commits {
			message := commit.Message
			if message != "" {
				message = strings.Split(message, "\n")[0]
			}

			author := commit.Author
			if author == "" {
				author = "Unknown"
			}

			qeLabel := formatQELabel(commit.QETestingLabel)
			result.WriteString(fmt.Sprintf("- %s (%s)%s\n", message, author, qeLabel))
		}
		result.WriteString("\n")

		result.WriteString("Files:\n")
		for _, file := range comparison.Files {
			filename := file.Filename
			if file.PreviousFilename != "" {
				filename = fmt.Sprintf("%s (renamed from %s)", file.Filename, file.PreviousFilename)
			}
			result.WriteString(fmt.Sprintf("- %s: %s +%d/-%d\n", filename, file.Status, file.Additions, file.Deletions))
		}

		result.WriteString(fmt.Sprintf("\nTotal: %d files, +%d/-%d lines\n",
			comparison.Stats.TotalFiles, comparison.Stats.TotalAdditions, comparison.Stats.TotalDeletions))

		// Only show diffs section if at least one file has a patch
		hasDiffs := false
		for _, file := range comparison.Files {
			if file.Patch != "" {
				hasDiffs = true
				break
			}
		}

		if hasDiffs {
			result.WriteString("\nDiffs:\n")
			for _, file := range comparison.Files {
				if file.Patch != "" {
					result.WriteString(fmt.Sprintf("\n%s:\n", file.Filename))
					result.WriteString(file.Patch)
					result.WriteString("\n")
				}
			}
		}

		if len(comparisons) > 1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// formatQELabel returns a formatted QE label suffix for commit lines
func formatQELabel(label string) string {
	switch label {
	case shared.LabelQETested:
		return " [QE Tested]"
	case shared.LabelNeedsQETesting:
		return " [Needs QE Testing]"
	default:
		return ""
	}
}
