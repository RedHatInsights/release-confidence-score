package rest

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
