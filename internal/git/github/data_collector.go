package github

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"release-confidence-score/internal/changelog"
	"release-confidence-score/internal/shared"

	"github.com/google/go-github/v76/github"
)

// CompareData holds raw comparison data that can be truncated and reformatted
type CompareData struct {
	Comparison *github.CommitsComparison
	AllCommits []*github.RepositoryCommit
	CompareURL string
}

// FetchCompareDataWithMeta fetches GitHub compare data with PR analysis for user guidance and labels
// Returns: formatted diff, changelog, user guidance, documentation, compare data, error
func FetchCompareDataWithMeta(client *github.Client, compareURL string) (string, *changelog.Changelog, []shared.UserGuidance, *RepoDocumentation, *CompareData, error) {
	// Parse compare URL once
	owner, repo, baseCommit, headCommit, err := parseCompareURL(compareURL)
	if err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("failed to parse GitHub URL: %w", err)
	}

	// Fetch comparison data and all commits in one API call
	comparison, allCommits, err := FetchComparisonWithAllCommits(client, owner, repo, baseCommit, headCommit)
	if err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("failed to fetch GitHub compare data: %w", err)
	}

	var result strings.Builder

	// Format comparison data for LLM consumption
	formattedDiff := FormatComparisonForLLM(comparison, allCommits, compareURL)
	result.WriteString(formattedDiff)

	// Debug logging for GitHub API diff content
	slog.Debug("GitHub API diff content",
		"url", compareURL,
		"content_length", len(formattedDiff))

	// Try to fetch repository documentation
	docsFetcher := NewDocumentationFetcher(client)
	docs, err := docsFetcher.FetchCompleteDocsParsed(context.Background(), owner, repo)
	var documentation *RepoDocumentation
	if err == nil {
		documentation = docs
	} else {
		slog.Debug("Failed to fetch repository documentation", "error", err)
	}

	// Create GitHub API cache to avoid duplicate API calls
	// Both changelog building and user guidance extraction need PR analysis,
	// and multiple commits often share the same PR
	githubCache := newGithubCache()

	// Build changelog from commits (metadata only - PR numbers and QE labels)
	changelogData := buildChangelog(allCommits, compareURL, client, owner, repo, githubCache)

	// Extract user guidance from commits (completely separate concern)
	userGuidance := extractUserGuidanceFromCommits(allCommits, client, owner, repo, githubCache)

	// Store raw comparison data for potential truncation
	compareData := &CompareData{
		Comparison: comparison,
		AllCommits: allCommits,
		CompareURL: compareURL,
	}

	return result.String(), changelogData, userGuidance, documentation, compareData, nil
}

// buildChangelog creates a Changelog from GitHub commits with PR enrichment for metadata
// Returns only the changelog - user guidance is handled separately
func buildChangelog(allCommits []*github.RepositoryCommit, compareURL string, client *github.Client, owner, repo string, githubCache *githubCache) *changelog.Changelog {
	changelogData := &changelog.Changelog{
		RepoURL: compareURL[:strings.Index(compareURL, "/compare/")],
		DiffURL: compareURL,
		Commits: make([]changelog.Commit, 0),
	}

	slog.Debug("Building changelog with PR enrichment", "owner", owner, "repo", repo, "commit_count", len(allCommits))

	// Process all collected commits with PR enrichment for metadata
	for _, commit := range allCommits {
		entry := buildChangelogEntry(commit, client, owner, repo, changelogData.RepoURL, githubCache)
		if entry != nil {
			changelogData.Commits = append(changelogData.Commits, *entry)
		}
	}

	slog.Debug("Built changelog with PR metadata",
		"commits", len(changelogData.Commits))

	return changelogData
}

// buildChangelogEntry creates a Commit from a GitHub commit with PR enrichment for metadata
// Returns only the changelog entry - user guidance is handled separately
func buildChangelogEntry(commit *github.RepositoryCommit, client *github.Client, owner, repo, repoURL string, githubCache *githubCache) *changelog.Commit {
	if commit.SHA == nil {
		return nil
	}

	entry := &changelog.Commit{
		SHA:      *commit.SHA,
		ShortSHA: (*commit.SHA)[:8],
		Message:  "No message",
		Author:   "Unknown",
	}

	// Extract commit message (first line only)
	if commit.Commit != nil && commit.Commit.Message != nil {
		lines := strings.Split(*commit.Commit.Message, "\n")
		if len(lines) > 0 {
			entry.Message = strings.TrimSpace(lines[0])
		}
	}

	// Extract author name
	if commit.Commit != nil && commit.Commit.Author != nil {
		if commit.Commit.Author.Name != nil {
			entry.Author = *commit.Commit.Author.Name
		}
	}

	// Enrich with PR metadata (PR number and QE label)
	slog.Debug("Processing commit for changelog", "commit", entry.ShortSHA, "sha", entry.SHA)

	// Find PR for this commit (cached to avoid duplicate API calls)
	prNumber, err := githubCache.getOrFetchPRForCommit(client, owner, repo, entry.SHA)
	if err != nil {
		slog.Warn("Failed to find PR for commit", "commit", entry.ShortSHA, "error", err)
		return entry
	}

	if prNumber == 0 {
		slog.Debug("No PR found for commit", "commit", entry.ShortSHA)
		return entry
	}

	slog.Debug("Found PR for commit", "commit", entry.ShortSHA, "pr", prNumber)
	entry.PRNumber = prNumber

	// Get PR object from cache (avoids duplicate API calls)
	pr, err := githubCache.getOrFetchPR(client, owner, repo, prNumber)
	if err != nil {
		slog.Warn("Failed to get PR object", "pr", prNumber, "error", err)
		return entry
	}

	// Extract QE testing label from PR
	qeLabel := GetPRQELabel(pr)
	entry.QETestingLabel = qeLabel

	slog.Debug("Enriched commit with PR metadata",
		"commit", entry.ShortSHA,
		"pr", prNumber,
		"qe_label", qeLabel)

	return entry
}

// extractUserGuidanceFromCommits extracts all user guidance from PR comments associated with commits
// This is a separate concern from changelog building and iterates over commits independently
func extractUserGuidanceFromCommits(allCommits []*github.RepositoryCommit, client *github.Client, owner, repo string, githubCache *githubCache) []shared.UserGuidance {
	var allUserGuidance []shared.UserGuidance

	slog.Debug("Extracting user guidance from commits", "owner", owner, "repo", repo, "commit_count", len(allCommits))

	for _, commit := range allCommits {
		if commit.SHA == nil {
			continue
		}

		sha := *commit.SHA
		shortSHA := sha[:8]

		// Find PR for this commit (cached to avoid duplicate API calls)
		prNumber, err := githubCache.getOrFetchPRForCommit(client, owner, repo, sha)
		if err != nil {
			slog.Warn("Failed to find PR for commit (user guidance)", "commit", shortSHA, "error", err)
			continue
		}

		if prNumber == 0 {
			slog.Debug("No PR found for commit (user guidance)", "commit", shortSHA)
			continue
		}

		slog.Debug("Extracting user guidance from PR", "commit", shortSHA, "pr", prNumber)

		// Get PR object from cache (avoids duplicate API calls)
		pr, err := githubCache.getOrFetchPR(client, owner, repo, prNumber)
		if err != nil {
			slog.Warn("Failed to get PR object", "pr", prNumber, "error", err)
			continue
		}

		// Extract user guidance from PR (cached to avoid duplicate API calls)
		userGuidance, err := GetPRUserGuidance(client, owner, repo, pr, githubCache)
		if err != nil {
			slog.Warn("Failed to extract user guidance", "pr", prNumber, "error", err)
			continue
		}

		if len(userGuidance) > 0 {
			// Count authorized vs unauthorized for logging
			authorized, unauthorized := 0, 0
			for _, guidance := range userGuidance {
				if guidance.IsAuthorized {
					authorized++
				} else {
					unauthorized++
				}
			}

			slog.Debug("Found user guidance in PR",
				"commit", shortSHA,
				"pr", prNumber,
				"authorized_guidance", authorized,
				"unauthorized_guidance", unauthorized)

			allUserGuidance = append(allUserGuidance, userGuidance...)
		}
	}

	slog.Debug("User guidance extraction complete",
		"total_guidance", len(allUserGuidance))

	return allUserGuidance
}
