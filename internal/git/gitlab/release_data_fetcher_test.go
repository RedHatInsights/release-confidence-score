package gitlab

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
			name: "simple project with SHA refs",
			url:  "https://gitlab.com/owner/repo/-/compare/abc123...def456",
			want: true,
		},
		{
			name: "version tags",
			url:  "https://gitlab.com/gitlab-org/api/client-go/-/compare/v1.9.0...v1.9.1",
			want: true,
		},
		{
			name: "branch names",
			url:  "https://gitlab.com/owner/repo/-/compare/main...feature-branch",
			want: true,
		},
		{
			name: "nested group",
			url:  "https://gitlab.com/group/subgroup/repo/-/compare/v1.0.0...v2.0.0",
			want: true,
		},
		{
			name: "deeply nested group",
			url:  "https://gitlab.com/a/b/c/d/repo/-/compare/main...develop",
			want: true,
		},
		{
			name: "self-hosted GitLab",
			url:  "https://gitlab.example.com/team/project/-/compare/v1.0...v2.0",
			want: true,
		},
		{
			name: "http scheme",
			url:  "http://gitlab.com/owner/repo/-/compare/v1.0.0...v2.0.0",
			want: true,
		},
		{
			name: "with query params",
			url:  "https://gitlab.com/owner/repo/-/compare/v1.0.0...v2.0.0?from_project_id=123",
			want: true,
		},

		// Invalid URLs
		{
			name: "github URL",
			url:  "https://github.com/owner/repo/compare/v1.0.0...v2.0.0",
			want: false,
		},
		{
			name: "missing -/compare path",
			url:  "https://gitlab.com/owner/repo/compare/v1.0.0...v2.0.0",
			want: false,
		},
		{
			name: "wrong separator",
			url:  "https://gitlab.com/owner/repo/-/compare/v1.0.0..v2.0.0",
			want: false,
		},
		{
			name: "merge request URL",
			url:  "https://gitlab.com/owner/repo/-/merge_requests/123",
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

func TestGitlabCompareRegexExtraction(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantHost    string
		wantProject string
		wantBase    string
		wantHead    string
	}{
		{
			name:        "simple project",
			url:         "https://gitlab.com/owner/repo/-/compare/abc123...def456",
			wantHost:    "gitlab.com",
			wantProject: "owner/repo",
			wantBase:    "abc123",
			wantHead:    "def456",
		},
		{
			name:        "version tags",
			url:         "https://gitlab.com/gitlab-org/api/client-go/-/compare/v1.9.0...v1.9.1",
			wantHost:    "gitlab.com",
			wantProject: "gitlab-org/api/client-go",
			wantBase:    "v1.9.0",
			wantHead:    "v1.9.1",
		},
		{
			name:        "nested group",
			url:         "https://gitlab.com/group/subgroup/repo/-/compare/main...develop",
			wantHost:    "gitlab.com",
			wantProject: "group/subgroup/repo",
			wantBase:    "main",
			wantHead:    "develop",
		},
		{
			name:        "self-hosted",
			url:         "https://gitlab.cee.redhat.com/team/project/-/compare/v1.0...v2.0",
			wantHost:    "gitlab.cee.redhat.com",
			wantProject: "team/project",
			wantBase:    "v1.0",
			wantHead:    "v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := gitlabCompareRegex.FindStringSubmatch(tt.url)
			if len(matches) != 5 {
				t.Fatalf("expected 5 matches, got %d: %v", len(matches), matches)
			}

			if matches[1] != tt.wantHost {
				t.Errorf("host = %q, want %q", matches[1], tt.wantHost)
			}
			if matches[2] != tt.wantProject {
				t.Errorf("project = %q, want %q", matches[2], tt.wantProject)
			}
			if matches[3] != tt.wantBase {
				t.Errorf("base = %q, want %q", matches[3], tt.wantBase)
			}
			if matches[4] != tt.wantHead {
				t.Errorf("head = %q, want %q", matches[4], tt.wantHead)
			}
		})
	}
}
