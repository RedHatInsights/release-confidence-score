package shared

import (
	"log/slog"

	"release-confidence-score/internal/git/types"
)

// LabeledCommit represents a commit with its QE testing label
// Used for passing QE testing information to prompt templates
type LabeledCommit struct {
	ShortSHA       string
	Message        string
	RepoURL        string
	QETestingLabel string // "qe-tested", "needs-qe-testing", or empty
	PRNumber       int64
}

// CommitsByRepo groups commits by repository for cleaner display
type CommitsByRepo struct {
	RepoURL string
	Commits []string // list of short SHAs
}

// TestingCommits holds QE testing information grouped by status
type TestingCommits struct {
	Tested       []CommitsByRepo // Commits with qe-tested label
	NeedsTesting []CommitsByRepo // Commits with needs-qe-testing label
}

// BuildTestingCommits extracts and organizes QE testing information from comparisons
// Returns nil if no QE testing labels are found
func BuildTestingCommits(comparisons []*types.Comparison) *TestingCommits {
	// Separate commits by QE testing status
	var qeTestedCommits []LabeledCommit
	var needsQETestingCommits []LabeledCommit

	for _, cmp := range comparisons {
		for _, commit := range cmp.Commits {
			if commit.QETestingLabel == "" {
				continue
			}

			labeledCommit := LabeledCommit{
				ShortSHA:       commit.ShortSHA,
				Message:        commit.Message,
				RepoURL:        cmp.RepoURL,
				QETestingLabel: commit.QETestingLabel,
				PRNumber:       commit.PRNumber,
			}

			switch commit.QETestingLabel {
			case "qe-tested":
				qeTestedCommits = append(qeTestedCommits, labeledCommit)
			case "needs-qe-testing":
				needsQETestingCommits = append(needsQETestingCommits, labeledCommit)
			}

			slog.Debug("Found QE labeled commit",
				"commit", commit.ShortSHA,
				"label", commit.QETestingLabel,
				"pr", commit.PRNumber)
		}
	}

	// Return nil if no labeled commits found
	if len(qeTestedCommits) == 0 && len(needsQETestingCommits) == 0 {
		return nil
	}

	slog.Debug("QE testing coverage summary",
		"qe_tested", len(qeTestedCommits),
		"needs_testing", len(needsQETestingCommits),
		"total_labeled", len(qeTestedCommits)+len(needsQETestingCommits))

	return &TestingCommits{
		Tested:       groupCommitsByRepo(qeTestedCommits),
		NeedsTesting: groupCommitsByRepo(needsQETestingCommits),
	}
}

// groupCommitsByRepo groups QE labeled commits by repository URL
func groupCommitsByRepo(commits []LabeledCommit) []CommitsByRepo {
	if len(commits) == 0 {
		return nil
	}

	// Group commits by repo URL
	repoMap := make(map[string][]string)
	repoOrder := []string{} // Track insertion order

	for _, commit := range commits {
		if _, exists := repoMap[commit.RepoURL]; !exists {
			repoOrder = append(repoOrder, commit.RepoURL)
		}
		repoMap[commit.RepoURL] = append(repoMap[commit.RepoURL], commit.ShortSHA)
	}

	// Build result preserving order
	result := make([]CommitsByRepo, 0, len(repoOrder))
	for _, repoURL := range repoOrder {
		result = append(result, CommitsByRepo{
			RepoURL: repoURL,
			Commits: repoMap[repoURL],
		})
	}

	return result
}
