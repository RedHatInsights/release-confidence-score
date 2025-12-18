package github

import "testing"

func TestIsCompareURL(t *testing.T) {
	f := &Fetcher{}

	tests := []struct {
		name string
		url  string
		want bool
	}{
		// Valid URLs
		{
			name: "SHA refs",
			url:  "https://github.com/owner/repo/compare/abc123...def456",
			want: true,
		},
		{
			name: "version tags",
			url:  "https://github.com/google/go-github/compare/v79.0.0...v80.0.0",
			want: true,
		},
		{
			name: "branch names",
			url:  "https://github.com/owner/repo/compare/main...feature-branch",
			want: true,
		},
		{
			name: "release tags with prefix",
			url:  "https://github.com/owner/repo/compare/release-1.2.3...release-1.2.4",
			want: true,
		},
		{
			name: "http scheme",
			url:  "http://github.com/owner/repo/compare/v1.0.0...v2.0.0",
			want: true,
		},
		{
			name: "mixed SHA and tag",
			url:  "https://github.com/owner/repo/compare/abc123def...v1.0.0",
			want: true,
		},

		// Invalid URLs
		{
			name: "not github.com",
			url:  "https://gitlab.com/owner/repo/compare/v1.0.0...v2.0.0",
			want: false,
		},
		{
			name: "missing compare path",
			url:  "https://github.com/owner/repo/v1.0.0...v2.0.0",
			want: false,
		},
		{
			name: "wrong separator",
			url:  "https://github.com/owner/repo/compare/v1.0.0..v2.0.0",
			want: false,
		},
		{
			name: "empty refs",
			url:  "https://github.com/owner/repo/compare/...",
			want: false,
		},
		{
			name: "pull request URL",
			url:  "https://github.com/owner/repo/pull/123",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.IsCompareURL(tt.url)
			if got != tt.want {
				t.Errorf("IsCompareURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseCompareURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantOwner  string
		wantRepo   string
		wantBase   string
		wantHead   string
		wantErr    bool
	}{
		{
			name:      "SHA refs",
			url:       "https://github.com/owner/repo/compare/abc123...def456",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantBase:  "abc123",
			wantHead:  "def456",
		},
		{
			name:      "version tags",
			url:       "https://github.com/google/go-github/compare/v79.0.0...v80.0.0",
			wantOwner: "google",
			wantRepo:  "go-github",
			wantBase:  "v79.0.0",
			wantHead:  "v80.0.0",
		},
		{
			name:      "branch with hyphen",
			url:       "https://github.com/org/my-repo/compare/main...feature-branch",
			wantOwner: "org",
			wantRepo:  "my-repo",
			wantBase:  "main",
			wantHead:  "feature-branch",
		},
		{
			name:    "invalid URL",
			url:     "https://github.com/owner/repo/pulls",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, base, head, err := parseCompareURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if head != tt.wantHead {
				t.Errorf("head = %q, want %q", head, tt.wantHead)
			}
		})
	}
}

func TestExtractRepoURL(t *testing.T) {
	tests := []struct {
		name       string
		compareURL string
		want       string
	}{
		{
			name:       "standard compare URL",
			compareURL: "https://github.com/owner/repo/compare/v1.0.0...v2.0.0",
			want:       "https://github.com/owner/repo",
		},
		{
			name:       "with SHA refs",
			compareURL: "https://github.com/org/my-repo/compare/abc123...def456",
			want:       "https://github.com/org/my-repo",
		},
		{
			name:       "no compare in URL",
			compareURL: "https://github.com/owner/repo",
			want:       "https://github.com/owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRepoURL(tt.compareURL)
			if got != tt.want {
				t.Errorf("extractRepoURL(%q) = %q, want %q", tt.compareURL, got, tt.want)
			}
		})
	}
}
