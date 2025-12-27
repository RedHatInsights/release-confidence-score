package shared

import "testing"

func TestParseCompareURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantBase  string
		wantHead  string
		wantErr   bool
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
		{
			name:    "wrong separator",
			url:     "https://github.com/owner/repo/compare/v1.0.0..v2.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, base, head, err := ParseCompareURL(tt.url)

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
			got := ExtractRepoURL(tt.compareURL)
			if got != tt.want {
				t.Errorf("ExtractRepoURL(%q) = %q, want %q", tt.compareURL, got, tt.want)
			}
		})
	}
}

func TestCompareURLRegex(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid https", "https://github.com/owner/repo/compare/v1...v2", true},
		{"valid http", "http://github.com/owner/repo/compare/v1...v2", true},
		{"not github", "https://gitlab.com/owner/repo/compare/v1...v2", false},
		{"missing compare", "https://github.com/owner/repo/v1...v2", false},
		{"wrong separator", "https://github.com/owner/repo/compare/v1..v2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareURLRegex.MatchString(tt.url)
			if got != tt.want {
				t.Errorf("CompareURLRegex.MatchString(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
