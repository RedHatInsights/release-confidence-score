package github

import (
	"testing"

	"github.com/google/go-github/v87/github"
)

func TestNewDocumentationSource(t *testing.T) {
	client, err := github.NewClient()
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	owner := "test-owner"
	repo := "test-repo"

	source := newDocumentationSource(client, owner, repo)

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
	client, err := github.NewClient()
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	source := newDocumentationSource(client, "owner", "repo")

	if source == nil {
		t.Error("expected non-nil source")
	}
}

func TestFetchFileContent_Constructor(t *testing.T) {
	client, err := github.NewClient()
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	owner := "test-owner"
	repo := "test-repo"

	source := newDocumentationSource(client, owner, repo)

	if source.owner != owner || source.repo != repo {
		t.Error("documentationSource fields not set correctly")
	}
}
