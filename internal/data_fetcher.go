package internal

import (
	"log/slog"

	"release-confidence-score/internal/changelog"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/github"
	"release-confidence-score/internal/shared"

	githubapi "github.com/google/go-github/v79/github"
)

// GetReleaseData fetches raw release data from multiple GitHub compare URLs
// Returns: changelogs, GitHub user guidance, documentation, comparisons, error
func GetReleaseData(githubClient *githubapi.Client, cfg *config.Config, urls []string) ([]*changelog.Changelog, []shared.UserGuidance, []*github.RepoDocumentation, []*github.CompareData, error) {
	if len(urls) == 0 {
		return []*changelog.Changelog{}, []shared.UserGuidance{}, []*github.RepoDocumentation{}, []*github.CompareData{}, nil
	}

	var changelogs []*changelog.Changelog
	var allUserGuidance []shared.UserGuidance
	var documentation []*github.RepoDocumentation
	var comparisons []*github.CompareData

	for _, url := range urls {
		// Only GitHub compare URLs are supported
		if !github.IsGitHubCompareURL(url) {
			slog.Warn("Skipping non-GitHub URL", "url", url)
			continue
		}

		slog.Debug("Fetching GitHub data", "url", url)
		// Fetch raw comparison data, changelog, user guidance, and documentation
		_, changelog, userGuidance, docs, compareData, err := github.FetchCompareDataWithMeta(githubClient, cfg, url)
		if err != nil {
			slog.Error("❌ Error fetching GitHub data", "error", err, "url", url)
			continue
		}

		// Collect changelog if available
		if changelog != nil {
			slog.Debug("✅ Collected changelog",
				"commit_count", len(changelog.Commits))
			changelogs = append(changelogs, changelog)
		} else {
			slog.Warn("⚠️ No changelog data received")
		}

		// Collect user guidance from this repo
		if len(userGuidance) > 0 {
			slog.Debug("✅ Collected user guidance from GitHub",
				"count", len(userGuidance))
			allUserGuidance = append(allUserGuidance, userGuidance...)
		}

		// Collect documentation metadata if available
		if docs != nil && docs.EntryPointFile != "" {
			slog.Debug("📚 Collected documentation",
				"repo_url", docs.RepoURL,
				"entry_point", docs.EntryPointFile)
			documentation = append(documentation, docs)
		}

		// Store comparison data (always present for successful fetch)
		if compareData != nil {
			comparisons = append(comparisons, compareData)
		}
	}

	return changelogs, allUserGuidance, documentation, comparisons, nil
}
