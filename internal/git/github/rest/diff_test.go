package rest

import (
	"testing"

	"github.com/google/go-github/v80/github"
)

func TestExtractQELabel(t *testing.T) {
	tests := []struct {
		name     string
		pr       *github.PullRequest
		expected string
	}{
		{
			name:     "nil PR",
			pr:       nil,
			expected: "",
		},
		{
			name:     "no labels",
			pr:       &github.PullRequest{Labels: []*github.Label{}},
			expected: "",
		},
		{
			name: "qe-tested label",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("rcs/qe-tested")},
				},
			},
			expected: "rcs/qe-tested",
		},
		{
			name: "needs-qe-testing label",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("rcs/needs-qe-testing")},
				},
			},
			expected: "rcs/needs-qe-testing",
		},
		{
			name: "both labels - qe-tested wins",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("rcs/needs-qe-testing")},
					{Name: github.Ptr("rcs/qe-tested")},
				},
			},
			expected: "rcs/qe-tested",
		},
		{
			name: "unrelated labels",
			pr: &github.PullRequest{
				Labels: []*github.Label{
					{Name: github.Ptr("bug")},
					{Name: github.Ptr("enhancement")},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractQELabel(tt.pr)
			if result != tt.expected {
				t.Errorf("extractQELabel() = %q, want %q", result, tt.expected)
			}
		})
	}
}
