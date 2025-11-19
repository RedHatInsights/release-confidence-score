package github

import (
	"fmt"
	"strings"

	"github.com/google/go-github/v76/github"
)

// FormatComparisonForLLM converts GitHub comparison data to LLM-friendly format
func FormatComparisonForLLM(comparison *github.CommitsComparison, allCommits []*github.RepositoryCommit, originalURL string) string {
	var result strings.Builder

	// Header with source information
	result.WriteString("=== GitHub Comparison Data (SDK) ===\n")
	result.WriteString(fmt.Sprintf("Source URL: %s\n", originalURL))

	// Extract basic comparison info
	if comparison.Status != nil {
		result.WriteString(fmt.Sprintf("Status: %s\n", *comparison.Status))
	}
	if comparison.AheadBy != nil {
		result.WriteString(fmt.Sprintf("Ahead by: %d commits\n", *comparison.AheadBy))
	}
	if comparison.BehindBy != nil {
		result.WriteString(fmt.Sprintf("Behind by: %d commits\n", *comparison.BehindBy))
	}
	if comparison.TotalCommits != nil {
		result.WriteString(fmt.Sprintf("Total commits: %d\n", *comparison.TotalCommits))
	}

	// File information
	if comparison.Files != nil {
		result.WriteString(fmt.Sprintf("Files changed: %d\n\n", len(comparison.Files)))

		// Commit summary
		if allCommits != nil && len(allCommits) > 0 {
			result.WriteString("=== Commits ===\n")
			for _, commit := range allCommits {
				if commit.SHA != nil && len(*commit.SHA) >= 8 {
					shortSHA := (*commit.SHA)[:8]
					message := "No message"
					author := "Unknown"

					if commit.Commit != nil && commit.Commit.Message != nil {
						message = strings.Split(*commit.Commit.Message, "\n")[0] // First line only
					}
					if commit.Commit != nil && commit.Commit.Author != nil && commit.Commit.Author.Name != nil {
						author = *commit.Commit.Author.Name
					}

					result.WriteString(fmt.Sprintf("- %s: %s (%s)\n", shortSHA, message, author))
				}
			}
			result.WriteString("\n")
		}

		// File changes summary
		result.WriteString("=== File Changes Summary ===\n")
		totalAdditions := 0
		totalDeletions := 0

		for _, file := range comparison.Files {
			filename := "unknown"
			if file.Filename != nil {
				filename = *file.Filename
			}

			additions := 0
			if file.Additions != nil {
				additions = *file.Additions
				totalAdditions += additions
			}

			deletions := 0
			if file.Deletions != nil {
				deletions = *file.Deletions
				totalDeletions += deletions
			}

			status := "unknown"
			if file.Status != nil {
				status = *file.Status
			}

			result.WriteString(fmt.Sprintf("- %s: +%d/-%d (%s)\n", filename, additions, deletions, status))
		}
		result.WriteString(fmt.Sprintf("\nTotal: +%d/-%d lines\n\n", totalAdditions, totalDeletions))

		// Detailed file diffs
		result.WriteString("=== Detailed File Diffs ===\n")
		for i, file := range comparison.Files {
			filename := "unknown"
			if file.Filename != nil {
				filename = *file.Filename
			}

			if file.Patch != nil && *file.Patch != "" {
				additions := 0
				if file.Additions != nil {
					additions = *file.Additions
				}

				deletions := 0
				if file.Deletions != nil {
					deletions = *file.Deletions
				}

				status := "unknown"
				if file.Status != nil {
					status = *file.Status
				}

				result.WriteString(fmt.Sprintf("\n--- File %d: %s ---\n", i+1, filename))
				result.WriteString(fmt.Sprintf("Status: %s (+%d/-%d)\n", status, additions, deletions))
				result.WriteString("Diff:\n")
				result.WriteString(*file.Patch)
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

// FormatMultipleComparisons formats multiple GitHub comparisons into a single diff string
func FormatMultipleComparisons(comparisons []*CompareData) string {
	if len(comparisons) == 0 {
		return ""
	}

	var result strings.Builder
	for i, compareData := range comparisons {
		if compareData == nil || compareData.Comparison == nil {
			continue
		}

		result.WriteString(fmt.Sprintf("=== Diff %d: %s ===\n", i+1, compareData.CompareURL))
		formattedDiff := FormatComparisonForLLM(compareData.Comparison, compareData.AllCommits, compareData.CompareURL)
		result.WriteString(formattedDiff)
		result.WriteString("\n\n")
	}

	return result.String()
}
