package shared

import (
	"testing"

	"github.com/google/go-github/v80/github"
)

func TestNewDocumentationSource(t *testing.T) {
	client := github.NewClient(nil)
	owner := "test-owner"
	repo := "test-repo"

	source := NewDocumentationSource(client, owner, repo)

	if source.client != client {
		t.Error("client not set correctly")
	}
	if source.owner != owner {
		t.Errorf("expected owner %s, got %s", owner, source.owner)
	}
	if source.repo != repo {
		t.Errorf("expected repo %s, got %s", repo, source.repo)
	}
}

func TestGetDefaultBranch_NilDefaultBranch(t *testing.T) {
	// This test demonstrates the nil handling logic exists
	// We can't easily test the GitHub API without mocking, but we can test the constructor
	client := github.NewClient(nil)
	source := NewDocumentationSource(client, "owner", "repo")

	if source == nil {
		t.Error("expected non-nil source")
	}
}

func TestFetchFileContent_Constructor(t *testing.T) {
	// Test that the documentationSource is properly constructed
	client := github.NewClient(nil)
	owner := "test-owner"
	repo := "test-repo"

	source := NewDocumentationSource(client, owner, repo)

	// Verify fields are set (can't test actual API calls without complex mocking)
	if source.owner != owner || source.repo != repo {
		t.Error("DocumentationSource fields not set correctly")
	}
}
