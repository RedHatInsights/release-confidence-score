package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v79/github"
)

// FetchFileContent gets file content from GitHub API using SDK
func FetchFileContent(client *github.Client, owner, repo, path string) (string, error) {
	ctx := context.Background()

	// Use GitHub SDK to fetch file content
	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", path, err)
	}

	// GitHub SDK automatically handles base64 decoding
	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode content for %s: %w", path, err)
	}

	return content, nil
}

// FetchComparisonWithAllCommits gets comparison data and all commits with pagination
func FetchComparisonWithAllCommits(client *github.Client, owner, repo, baseCommit, headCommit string) (*github.CommitsComparison, []*github.RepositoryCommit, error) {
	ctx := context.Background()
	page := 1
	perPage := 100
	var allCommits []*github.RepositoryCommit
	var comparisonData *github.CommitsComparison

	for {
		comparison, resp, err := client.Repositories.CompareCommits(ctx, owner, repo, baseCommit, headCommit,
			&github.ListOptions{Page: page, PerPage: perPage})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch GitHub compare data (page %d): %w", page, err)
		}

		// Store comparison data from first page
		if page == 1 {
			comparisonData = comparison
		}

		// Collect commits from this page
		if comparison.Commits != nil {
			allCommits = append(allCommits, comparison.Commits...)
		}

		// Check if we have more pages
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return comparisonData, allCommits, nil
}
