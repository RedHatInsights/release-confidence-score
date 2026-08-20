package shared

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	// GitLabBaseURL matches the test server so it's treated as the trusted
	// configured instance, bypassing the private-IP SSRF guard (the server
	// listens on loopback). SSRF blocking itself is covered by
	// TestFetchExternalURL_BlocksPrivateIPForUntrustedHost below.
	cfg := &config.Config{GitLabBaseURL: server.URL}
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
	cfg := &config.Config{GitLabBaseURL: server.URL}
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
		GitLabToken:   "test-token-123",
		GitLabBaseURL: server.URL, // matches the fetched host, so it's treated as the configured GitLab instance
	}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	_, err := fetcher.fetchExternalURL(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if receivedHeader != "test-token-123" {
		t.Errorf("expected PRIVATE-TOKEN header %q, got: %q", "test-token-123", receivedHeader)
	}
}

func TestFetchExternalURL_BlocksPrivateIPForUntrustedHost(t *testing.T) {
	// Create a test server (listens on loopback, a private/non-public address)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("should never be returned"))
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	// No GitLabBaseURL configured (or one that doesn't match), so the target
	// is treated as untrusted repo-controlled content and must be blocked.
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	_, err := fetcher.fetchExternalURL(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error when fetching an untrusted URL that resolves to a private address")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected error about blocked private address, got: %v", err)
	}
}

func TestFetchExternalURL_BlocksRedirectFromTrustedHostToPrivateTarget(t *testing.T) {
	// Simulates a compromised/open-redirect response from the trusted GitLab
	// host pointing at a different private-only target. The redirect must
	// still be blocked even though the original request went to a trusted host.
	internalTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("internal secret"))
	}))
	defer internalTarget.Close()

	// Use "localhost" so the redirect target is a distinct literal hostname
	// from the trusted GitLab server's "127.0.0.1", even though both resolve
	// to loopback.
	_, port, err := net.SplitHostPort(strings.TrimPrefix(internalTarget.URL, "http://"))
	if err != nil {
		t.Fatalf("failed to split internal target address: %v", err)
	}
	redirectTarget := "http://localhost:" + port

	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget, http.StatusFound)
	}))
	defer gitlabServer.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	cfg := &config.Config{GitLabBaseURL: gitlabServer.URL}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	_, err = fetcher.fetchExternalURL(context.Background(), gitlabServer.URL)

	if err == nil {
		t.Fatal("expected redirect from trusted host to a non-trusted private target to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected error mentioning blocked address, got: %v", err)
	}
}

func TestFetchExternalURL_ResponseBodyTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write more than maxResponseBodySize (1 MiB)
		buf := make([]byte, maxResponseBodySize+1)
		w.Write(buf)
	}))
	defer server.Close()

	source := &mockDocumentationSource{}
	repo := types.Repository{}
	// GitLabBaseURL matches the test server so it's treated as the trusted
	// configured instance, bypassing the private-IP SSRF guard (the server
	// listens on loopback). SSRF blocking itself is covered by
	// TestFetchExternalURL_BlocksPrivateIPForUntrustedHost above.
	cfg := &config.Config{GitLabBaseURL: server.URL}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	_, err := fetcher.fetchExternalURL(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error for oversized response body")
	}
	if !strings.Contains(err.Error(), "exceeds 1 MiB limit") {
		t.Errorf("expected error about exceeding limit, got: %v", err)
	}
}

func TestIsGitLabURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		gitlabHost string
		expected   bool
	}{
		{"configured instance matches", "https://gitlab.corp.example.com/user/repo", "gitlab.corp.example.com", true},
		{"case insensitive match", "https://GitLab.Corp.Example.COM/user/repo", "gitlab.corp.example.com", true},
		{"attacker-controlled gitlab prefix", "https://gitlab.evil.example/doc.md", "gitlab.corp.example.com", false},
		{"no host configured", "https://gitlab.com/user/repo", "", false},
		{"github.com", "https://github.com/user/repo", "gitlab.corp.example.com", false},
		{"random domain", "https://example.com/user/repo", "gitlab.corp.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGitLabURL(tt.url, tt.gitlabHost)
			if result != tt.expected {
				t.Errorf("isGitLabURL(%s, %s) = %v, expected %v", tt.url, tt.gitlabHost, result, tt.expected)
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

func TestGitlabHostname(t *testing.T) {
	tests := []struct {
		name       string
		gitlabBase string
		expected   string
	}{
		{"plain host", "https://gitlab.corp.example.com", "gitlab.corp.example.com"},
		{"path in base is ignored", "https://gitlab.corp.example.com/api/v4", "gitlab.corp.example.com"},
		{"case is normalized", "https://GitLab.Corp.Example.COM", "gitlab.corp.example.com"},
		{"empty base", "", ""},
		{"malformed base", "://invalid-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gitlabHostname(tt.gitlabBase)
			if result != tt.expected {
				t.Errorf("gitlabHostname(%s) = %q, expected %q", tt.gitlabBase, result, tt.expected)
			}
		})
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
	cfg := &config.Config{GitLabBaseURL: server.URL}
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
	cfg := &config.Config{GitLabBaseURL: server.URL}
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
	cfg := &config.Config{GitLabBaseURL: server.URL}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	blobURL := server.URL + "/blob/main/file.md"
	_, err := fetcher.fetchAdditionalDocContent(context.Background(), "main", blobURL)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
