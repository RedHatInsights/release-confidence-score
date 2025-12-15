package user

import (
	"strings"
	"testing"
	"time"

	"release-confidence-score/internal/git/types"
	"release-confidence-score/internal/llm/truncation"
)

func TestExtractAuthorizedGuidance(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.UserGuidance
		expected []string
	}{
		{
			name:     "empty input",
			input:    []types.UserGuidance{},
			expected: nil,
		},
		{
			name: "all authorized",
			input: []types.UserGuidance{
				{Content: "guidance 1", IsAuthorized: true},
				{Content: "guidance 2", IsAuthorized: true},
			},
			expected: []string{"guidance 1", "guidance 2"},
		},
		{
			name: "all unauthorized",
			input: []types.UserGuidance{
				{Content: "guidance 1", IsAuthorized: false},
				{Content: "guidance 2", IsAuthorized: false},
			},
			expected: nil,
		},
		{
			name: "mixed authorized and unauthorized",
			input: []types.UserGuidance{
				{Content: "authorized 1", IsAuthorized: true},
				{Content: "unauthorized 1", IsAuthorized: false},
				{Content: "authorized 2", IsAuthorized: true},
				{Content: "unauthorized 2", IsAuthorized: false},
			},
			expected: []string{"authorized 1", "authorized 2"},
		},
		{
			name: "single authorized",
			input: []types.UserGuidance{
				{Content: "only authorized", IsAuthorized: true},
			},
			expected: []string{"only authorized"},
		},
		{
			name: "single unauthorized",
			input: []types.UserGuidance{
				{Content: "only unauthorized", IsAuthorized: false},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAuthorizedGuidance(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("extractAuthorizedGuidance() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("extractAuthorizedGuidance()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestRenderUserPrompt(t *testing.T) {
	tests := []struct {
		name               string
		diff               string
		documentation      string
		userGuidance       []types.UserGuidance
		truncationMetadata truncation.TruncationMetadata
		expectInOutput     []string
		expectNotInOutput  []string
	}{
		{
			name:          "minimal prompt",
			diff:          "diff content",
			documentation: "",
			userGuidance:  nil,
			truncationMetadata: truncation.TruncationMetadata{
				Truncated: false,
			},
			expectInOutput: []string{
				"Analyze these code changes",
				"Code Changes",
				"diff content",
				"Provide your analysis",
			},
			expectNotInOutput: []string{
				"Truncation",
				"Additional Analysis Guidance",
				"Documentation",
			},
		},
		{
			name:          "with documentation",
			diff:          "diff content",
			documentation: "# Documentation\n\nThis is the documentation.",
			userGuidance:  nil,
			truncationMetadata: truncation.TruncationMetadata{
				Truncated: false,
			},
			expectInOutput: []string{
				"diff content",
				"## Documentation",
				"# Documentation",
				"This is the documentation.",
			},
		},
		{
			name: "with authorized user guidance",
			diff: "diff content",
			userGuidance: []types.UserGuidance{
				{Content: "Check the security", IsAuthorized: true},
				{Content: "Not included", IsAuthorized: false},
				{Content: "Review error handling", IsAuthorized: true},
			},
			truncationMetadata: truncation.TruncationMetadata{
				Truncated: false,
			},
			expectInOutput: []string{
				"Additional Analysis Guidance",
				"Check the security",
				"Review error handling",
			},
			expectNotInOutput: []string{
				"Not included",
			},
		},
		{
			name: "with truncation metadata",
			diff: "diff content",
			truncationMetadata: truncation.TruncationMetadata{
				Truncated:      true,
				Level:          "moderate",
				TotalFiles:     100,
				FilesPreserved: 80,
				FilesTruncated: 20,
			},
			expectInOutput: []string{
				"Truncation Applied",
				"moderate",
				"80/100",
				"20 files",
				"lines omitted",
				"metadata preserved",
			},
		},
		{
			name:          "full prompt with all features",
			diff:          "comprehensive diff content",
			documentation: "Complete documentation",
			userGuidance: []types.UserGuidance{
				{Content: "Important guidance", IsAuthorized: true},
			},
			truncationMetadata: truncation.TruncationMetadata{
				Truncated:      true,
				Level:          "aggressive",
				TotalFiles:     200,
				FilesPreserved: 50,
				FilesTruncated: 150,
			},
			expectInOutput: []string{
				"comprehensive diff content",
				"Truncation Applied",
				"aggressive",
				"50/200",
				"Additional Analysis Guidance",
				"Important guidance",
				"## Documentation",
				"Complete documentation",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderUserPrompt(tt.diff, tt.documentation, tt.userGuidance, tt.truncationMetadata)
			if err != nil {
				t.Fatalf("RenderUserPrompt() error = %v", err)
			}

			if result == "" {
				t.Error("RenderUserPrompt() returned empty string")
			}

			// Check expected content is present
			for _, expected := range tt.expectInOutput {
				if !strings.Contains(result, expected) {
					t.Errorf("RenderUserPrompt() output missing expected content: %q", expected)
				}
			}

			// Check unexpected content is not present
			for _, notExpected := range tt.expectNotInOutput {
				if strings.Contains(result, notExpected) {
					t.Errorf("RenderUserPrompt() output contains unexpected content: %q", notExpected)
				}
			}
		})
	}
}

func TestRenderUserPromptTemplateFormat(t *testing.T) {
	// Test that the template produces valid markdown structure
	diff := "sample diff"
	result, err := RenderUserPrompt(diff, "", nil, truncation.TruncationMetadata{})
	if err != nil {
		t.Fatalf("RenderUserPrompt() error = %v", err)
	}

	// Check for proper markdown heading structure
	if !strings.Contains(result, "## Code Changes") {
		t.Error("RenderUserPrompt() missing '## Code Changes' heading")
	}

	// Check it ends with the closing instruction
	if !strings.HasSuffix(strings.TrimSpace(result), "Provide your analysis in the exact JSON format specified in the system prompt. Include all required fields and ensure the JSON is valid.") {
		t.Error("RenderUserPrompt() missing closing instruction")
	}
}

func TestRenderUserPromptNoTruncationWhenNotTruncated(t *testing.T) {
	// Test that truncation metadata is omitted when Truncated = false
	diff := "diff content"
	truncationMetadata := truncation.TruncationMetadata{
		Truncated:      false,
		Level:          "moderate",
		TotalFiles:     100,
		FilesPreserved: 80,
		FilesTruncated: 20,
	}

	result, err := RenderUserPrompt(diff, "", nil, truncationMetadata)
	if err != nil {
		t.Fatalf("RenderUserPrompt() error = %v", err)
	}

	// Should not contain truncation section
	if strings.Contains(result, "Analysis Limitations") {
		t.Error("RenderUserPrompt() included truncation section when Truncated = false")
	}
	if strings.Contains(result, "moderate") {
		t.Error("RenderUserPrompt() included truncation level when Truncated = false")
	}
}

func TestRenderUserPromptConsistency(t *testing.T) {
	// Verify that rendering the same input multiple times produces the same output
	diff := "consistent diff"
	documentation := "consistent docs"
	userGuidance := []types.UserGuidance{
		{Content: "consistent guidance", IsAuthorized: true},
	}

	result1, err := RenderUserPrompt(diff, documentation, userGuidance, truncation.TruncationMetadata{})
	if err != nil {
		t.Fatalf("RenderUserPrompt() first call error = %v", err)
	}

	result2, err := RenderUserPrompt(diff, documentation, userGuidance, truncation.TruncationMetadata{})
	if err != nil {
		t.Fatalf("RenderUserPrompt() second call error = %v", err)
	}

	if result1 != result2 {
		t.Error("RenderUserPrompt() produced different results for identical inputs")
	}
}

func TestExtractAuthorizedGuidanceWithComplexData(t *testing.T) {
	// Test with more realistic UserGuidance data
	input := []types.UserGuidance{
		{
			Content:      "Please check for SQL injection vulnerabilities",
			Author:       "security-team",
			Date:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			CommentURL:   "https://github.com/user/repo/pull/1#comment1",
			IsAuthorized: true,
		},
		{
			Content:      "This looks good to me",
			Author:       "random-user",
			Date:         time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			CommentURL:   "https://github.com/user/repo/pull/1#comment2",
			IsAuthorized: false,
		},
		{
			Content:      "Verify the error handling is comprehensive",
			Author:       "lead-dev",
			Date:         time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
			CommentURL:   "https://github.com/user/repo/pull/1#comment3",
			IsAuthorized: true,
		},
	}

	result := extractAuthorizedGuidance(input)

	if len(result) != 2 {
		t.Errorf("extractAuthorizedGuidance() length = %d, want 2", len(result))
	}

	expected := []string{
		"Please check for SQL injection vulnerabilities",
		"Verify the error handling is comprehensive",
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("extractAuthorizedGuidance()[%d] = %q, want %q", i, v, expected[i])
		}
	}
}
