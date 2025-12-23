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

func TestParseCompareURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantHost    string
		wantProject string
		wantBase    string
		wantHead    string
		wantErr     bool
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
		{
			name:    "invalid URL",
			url:     "https://gitlab.com/owner/repo/-/merge_requests/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, project, base, head, err := parseCompareURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if project != tt.wantProject {
				t.Errorf("project = %q, want %q", project, tt.wantProject)
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

func TestSplitProjectPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantOwner string
		wantName  string
	}{
		{
			name:      "simple path",
			path:      "group/repo",
			wantOwner: "group",
			wantName:  "repo",
		},
		{
			name:      "nested group",
			path:      "group/subgroup/repo",
			wantOwner: "group/subgroup",
			wantName:  "repo",
		},
		{
			name:      "deeply nested",
			path:      "a/b/c/d/repo",
			wantOwner: "a/b/c/d",
			wantName:  "repo",
		},
		{
			name:      "no slash",
			path:      "repo",
			wantOwner: "",
			wantName:  "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, name := splitProjectPath(tt.path)
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestUrlEncodeProjectPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple path",
			path: "owner/repo",
			want: "owner%2Frepo",
		},
		{
			name: "nested group",
			path: "group/subgroup/repo",
			want: "group%2Fsubgroup%2Frepo",
		},
		{
			name: "no encoding needed",
			path: "repo",
			want: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := urlEncodeProjectPath(tt.path)
			if got != tt.want {
				t.Errorf("urlEncodeProjectPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
