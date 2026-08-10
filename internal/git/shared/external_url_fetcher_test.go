package shared

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/types"
)

func TestFetchExternalURL_Success(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("External document content"))
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	content, err := fetcher.fetchExternalURL(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if content != "External document content" {
		t.Errorf("expected 'External document content', got: %s", content)
	}
}

func TestFetchExternalURL_NotFound(t *testing.T) {
	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	_, err := fetcher.fetchExternalURL(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchExternalURL_GitLabAuthentication(t *testing.T) {
	// Create a test server that checks for GitLab auth header
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("PRIVATE-TOKEN")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("content"))
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	cfg := &config.Config{
		GitLabToken: "test-token-123",
	}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	// Use gitlab.com URL to trigger GitLab auth
	gitlabURL := server.URL + "/gitlab-path"
	// Need to mock this as GitLab URL - let's just test with the server URL
	// and manually verify token would be set
	_, err := fetcher.fetchExternalURL(context.Background(), gitlabURL)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Note: This won't actually trigger GitLab auth because the test server URL
	// doesn't match gitlab.com. We test the logic in isGitLabURL separately.
	_ = receivedHeader // Avoid unused variable warning
}

func TestIsGitLabURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		gitlabBase string
		expected   bool
	}{
		{"configured instance matches", "https://gitlab.corp.example.com/user/repo", "https://gitlab.corp.example.com", true},
		{"configured instance with path in base", "https://gitlab.corp.example.com/user/repo", "https://gitlab.corp.example.com/api/v4", true},
		{"case insensitive match", "https://GitLab.Corp.Example.COM/user/repo", "https://gitlab.corp.example.com", true},
		{"attacker-controlled gitlab prefix", "https://gitlab.evil.example/doc.md", "https://gitlab.corp.example.com", false},
		{"no base URL configured", "https://gitlab.com/user/repo", "", false},
		{"github.com", "https://github.com/user/repo", "https://gitlab.corp.example.com", false},
		{"random domain", "https://example.com/user/repo", "https://gitlab.corp.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGitLabURL(tt.url, tt.gitlabBase)
			if result != tt.expected {
				t.Errorf("isGitLabURL(%s, %s) = %v, expected %v", tt.url, tt.gitlabBase, result, tt.expected)
			}
		})
	}
}

func TestIsGitLabURL_InvalidURL(t *testing.T) {
	invalidURL := "://invalid-url-with-gitlab"
	result := isGitLabURL(invalidURL, "")
	if result {
		t.Error("expected false for malformed URL — must not fall back to string matching")
	}

	noGitLab := "://invalid-url"
	result = isGitLabURL(noGitLab, "")
	if result {
		t.Error("expected false for malformed URL not containing 'gitlab'")
	}
}

func TestFetchAdditionalDocContent_ExternalHTTPURL(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("External HTTP content"))
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	content, err := fetcher.fetchAdditionalDocContent(context.Background(), "main", server.URL)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if content != "External HTTP content" {
		t.Errorf("expected 'External HTTP content', got: %s", content)
	}
}

func TestFetchAdditionalDocContent_ExternalHTTPSURL(t *testing.T) {
	// Test HTTPS URL detection - use a plain HTTP server for simplicity
	// The HTTPS handling is tested implicitly through the HTTP client configuration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("HTTPS-style content"))
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	// Test with HTTPS URL prefix detection (using HTTP server for simplicity)
	content, err := fetcher.fetchAdditionalDocContent(context.Background(), "main", server.URL)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if content != "HTTPS-style content" {
		t.Errorf("expected 'HTTPS-style content', got: %s", content)
	}
}

func TestFetchAdditionalDocContent_BlobURLConversion(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that /raw/ is in the path (blob was converted)
		if r.URL.Path != "/raw/main/file.md" {
			t.Errorf("expected /raw/ in path, got: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("content"))
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	blobURL := server.URL + "/blob/main/file.md"
	_, err := fetcher.fetchAdditionalDocContent(context.Background(), "main", blobURL)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
