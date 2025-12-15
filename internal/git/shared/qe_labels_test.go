package shared

import "testing"

func TestExtractQELabel(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected string
	}{
		{"no labels", []string{}, ""},
		{"nil labels", nil, ""},
		{"only qe-tested", []string{"rcs/qe-tested"}, LabelQETested},
		{"only needs-qe-testing", []string{"rcs/needs-qe-testing"}, LabelNeedsQETesting},
		{"both labels - qe-tested wins", []string{"rcs/needs-qe-testing", "rcs/qe-tested"}, LabelQETested},
		{"mixed with other labels", []string{"bug", "rcs/qe-tested", "enhancement"}, LabelQETested},
		{"no matching labels", []string{"bug", "enhancement", "docs"}, ""},
		{"case insensitive", []string{"RCS/QE-TESTED"}, LabelQETested},
		{"case insensitive needs-testing", []string{"RCS/NEEDS-QE-TESTING"}, LabelNeedsQETesting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractQELabel(tt.labels)
			if result != tt.expected {
				t.Errorf("ExtractQELabel(%v) = %q, want %q", tt.labels, result, tt.expected)
			}
		})
	}
}
