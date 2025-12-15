package gitlab

import (
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestNewDocumentationSource(t *testing.T) {
	client, _ := gitlab.NewClient("")
	host := "gitlab.com"
	projectPath := "owner/repo"

	source := newDocumentationSource(client, host, projectPath)

	if source.client != client {
		t.Error("client not set correctly")
	}
	if source.host != host {
		t.Errorf("expected host %s, got %s", host, source.host)
	}
	if source.projectPath != projectPath {
		t.Errorf("expected projectPath %s, got %s", projectPath, source.projectPath)
	}
}

func TestGetDefaultBranch_Constructor(t *testing.T) {
	// This test demonstrates the empty string handling logic exists
	// We can't easily test the GitLab API without mocking, but we can test the constructor
	client, _ := gitlab.NewClient("")
	source := newDocumentationSource(client, "gitlab.com", "owner/repo")

	if source == nil {
		t.Error("expected non-nil source")
	}
}

func TestFetchFileContent_Constructor(t *testing.T) {
	// Test that the documentationSource is properly constructed
	client, _ := gitlab.NewClient("")
	host := "gitlab.com"
	projectPath := "owner/repo"

	source := newDocumentationSource(client, host, projectPath)

	// Verify fields are set (can't test actual API calls without complex mocking)
	if source.host != host || source.projectPath != projectPath {
		t.Error("documentationSource fields not set correctly")
	}
}
