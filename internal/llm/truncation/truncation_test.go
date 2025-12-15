package truncation

import (
	"strings"
	"testing"

	"release-confidence-score/internal/git/types"
)

func TestTruncateDocumentation(t *testing.T) {
	// Create sample documentation with linked docs
	docs := []*types.Documentation{
		{
			Repository: types.Repository{
				URL:           "https://github.com/user/repo",
				DefaultBranch: "main",
			},
			MainDocContent: "# Main README\n\nThis is the entry point.",
			MainDocFile:    "README.md",
			AdditionalDocsContent: map[string]string{
				"CONTRIBUTING.md": "# Contributing\n\nContribution guidelines.",
				"SECURITY.md":     "# Security\n\nSecurity policy.",
			},
			AdditionalDocsOrder: []string{"CONTRIBUTING.md", "SECURITY.md"},
		},
	}

	tests := []struct {
		name              string
		level             string
		expectedHasLinked bool
		description       string
	}{
		{
			name:              "low level preserves linked docs",
			level:             LevelLow,
			expectedHasLinked: true,
			description:       "Low truncation should keep all documentation",
		},
		{
			name:              "moderate level preserves linked docs",
			level:             LevelModerate,
			expectedHasLinked: true,
			description:       "Moderate truncation should keep all documentation",
		},
		{
			name:              "high level removes linked docs",
			level:             LevelHigh,
			expectedHasLinked: false,
			description:       "High truncation should remove linked docs",
		},
		{
			name:              "extreme level removes linked docs",
			level:             LevelExtreme,
			expectedHasLinked: false,
			description:       "Extreme truncation should remove linked docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateDocumentation(docs, tt.level)

			if len(result) != 1 {
				t.Fatalf("Expected 1 documentation set, got %d", len(result))
			}

			resultDoc := result[0]

			// Entry point should always be preserved
			if resultDoc.MainDocContent != docs[0].MainDocContent {
				t.Error("Entry point was modified")
			}

			// Check linked docs based on truncation level
			hasAdditionalDocs := len(resultDoc.AdditionalDocsContent) > 0
			if hasAdditionalDocs != tt.expectedHasLinked {
				t.Errorf("%s: expected hasLinkedDocs=%v, got %v (count=%d)",
					tt.description, tt.expectedHasLinked, hasAdditionalDocs, len(resultDoc.AdditionalDocsContent))
			}
		})
	}
}

func TestTruncateDocumentationNilHandling(t *testing.T) {
	tests := []struct {
		name  string
		input []*types.Documentation
	}{
		{
			name:  "nil slice",
			input: nil,
		},
		{
			name:  "empty slice",
			input: []*types.Documentation{},
		},
		{
			name:  "slice with nil entry",
			input: []*types.Documentation{nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateDocumentation(tt.input, LevelHigh)

			// Function should return a slice with the same length as input
			// (even if input is nil, result will be an empty slice - this is standard Go behavior)
			if len(result) != len(tt.input) {
				t.Errorf("Expected same length as input (%d), got %d", len(tt.input), len(result))
			}

			// For nil entries, verify they stay nil
			for i, doc := range tt.input {
				if doc == nil && result[i] != nil {
					t.Errorf("Expected nil entry at index %d to remain nil", i)
				}
			}
		})
	}
}

func TestTruncateDocumentationPreservesOriginal(t *testing.T) {
	// Create original documentation
	original := []*types.Documentation{
		{
			Repository: types.Repository{
				URL:           "https://github.com/user/repo",
				DefaultBranch: "main",
			},
			MainDocContent: "# README",
			MainDocFile:    "README.md",
			AdditionalDocsContent: map[string]string{
				"GUIDE.md": "# Guide",
			},
			AdditionalDocsOrder: []string{"GUIDE.md"},
		},
	}

	// Store original values
	originalLinkedDocsCount := len(original[0].AdditionalDocsContent)

	// Truncate with high level
	TruncateDocumentation(original, LevelHigh)

	// Verify original is unchanged
	if len(original[0].AdditionalDocsContent) != originalLinkedDocsCount {
		t.Error("Truncation modified the original documentation (should create a copy)")
	}
}

// Core truncation function tests

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 2},
		{"multiple lines", "line1\nline2\nline3", 3},
		{"trailing newline", "line1\nline2\n", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countLines(tt.input)
			if result != tt.expected {
				t.Errorf("countLines(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestClassifyFileRisk(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected FileRiskLevel
	}{
		// Critical risk files
		{"auth config", "config/auth.go", RiskCritical},
		{"database migration", "db/migrations/001_create_users.sql", RiskCritical},
		{"API spec", "openapi.yaml", RiskCritical},

		// Medium risk files (general application code)
		{"service core", "internal/service/processor.go", RiskMedium},
		{"API handler", "api/handlers/users.go", RiskMedium},

		// Medium risk files
		{"regular go file", "internal/utils/helper.go", RiskMedium},
		{"regular python", "src/main.py", RiskMedium},

		// Low risk files
		{"test file", "internal/service_test.go", RiskLow},
		{"doc file", "docs/README.md", RiskLow},
		{"generated file", "generated/api.pb.go", RiskLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyFileRisk(tt.filename)
			if result != tt.expected {
				t.Errorf("classifyFileRisk(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestShouldTruncateFile(t *testing.T) {
	tests := []struct {
		name     string
		risk     FileRiskLevel
		level    string
		expected bool
	}{
		// Critical files should NEVER be truncated
		{"critical/low", RiskCritical, LevelLow, false},
		{"critical/moderate", RiskCritical, LevelModerate, false},
		{"critical/high", RiskCritical, LevelHigh, false},
		{"critical/extreme", RiskCritical, LevelExtreme, false},

		// High risk files only truncated at high/extreme
		{"high/low", RiskHigh, LevelLow, false},
		{"high/moderate", RiskHigh, LevelModerate, false},
		{"high/high", RiskHigh, LevelHigh, true},
		{"high/extreme", RiskHigh, LevelExtreme, true},

		// Medium risk files truncated at moderate+
		{"medium/low", RiskMedium, LevelLow, false},
		{"medium/moderate", RiskMedium, LevelModerate, true},
		{"medium/high", RiskMedium, LevelHigh, true},
		{"medium/extreme", RiskMedium, LevelExtreme, true},

		// Low risk files always truncated
		{"low/low", RiskLow, LevelLow, true},
		{"low/moderate", RiskLow, LevelModerate, true},
		{"low/high", RiskLow, LevelHigh, true},
		{"low/extreme", RiskLow, LevelExtreme, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldTruncateFile(tt.risk, tt.level)
			if result != tt.expected {
				t.Errorf("shouldTruncateFile(%v, %s) = %v, want %v", tt.risk, tt.level, result, tt.expected)
			}
		})
	}
}

func TestTruncatePatch(t *testing.T) {
	// Create a patch with known line count
	lines := []string{}
	for i := 1; i <= 100; i++ {
		lines = append(lines, strings.Repeat("x", 50))
	}
	largePatch := strings.Join(lines, "\n")

	tests := []struct {
		name         string
		patch        string
		keepStart    int
		keepEnd      int
		expectOmit   bool
		checkContent bool
	}{
		{
			name:         "small patch not truncated",
			patch:        "line1\nline2\nline3",
			keepStart:    5,
			keepEnd:      5,
			expectOmit:   false,
			checkContent: true,
		},
		{
			name:         "large patch truncated",
			patch:        largePatch,
			keepStart:    10,
			keepEnd:      5,
			expectOmit:   true,
			checkContent: true,
		},
		{
			name:         "empty patch",
			patch:        "",
			keepStart:    10,
			keepEnd:      5,
			expectOmit:   false,
			checkContent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePatch(tt.patch, tt.keepStart, tt.keepEnd)

			if tt.expectOmit {
				if !strings.Contains(result, "lines omitted") {
					t.Error("Expected omission marker in truncated patch")
				}
			} else if tt.checkContent {
				if result != tt.patch {
					t.Error("Expected patch to remain unchanged")
				}
			}
		})
	}
}

func TestGetTruncationParams(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedStart int
		expectedEnd   int
	}{
		{"low level", LevelLow, 50, 20},
		{"moderate level", LevelModerate, 20, 10},
		{"high level", LevelHigh, 10, 5},
		{"extreme level", LevelExtreme, 5, 3},
		{"unknown level falls back to low", "unknown", 50, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := getTruncationParams(tt.level)
			if start != tt.expectedStart || end != tt.expectedEnd {
				t.Errorf("getTruncationParams(%s) = (%d, %d), want (%d, %d)",
					tt.level, start, end, tt.expectedStart, tt.expectedEnd)
			}
		})
	}
}

func TestGetSmallFileThreshold(t *testing.T) {
	tests := []struct {
		level    string
		expected int
	}{
		{LevelLow, SmallFileThresholdLow},
		{LevelModerate, SmallFileThresholdModerate},
		{LevelHigh, SmallFileThresholdHigh},
		{LevelExtreme, SmallFileThresholdExtreme},
		{"unknown", SmallFileThresholdLow},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			result := getSmallFileThreshold(tt.level)
			if result != tt.expected {
				t.Errorf("getSmallFileThreshold(%s) = %d, want %d", tt.level, result, tt.expected)
			}
		})
	}
}

func TestTruncateMultipleComparisons(t *testing.T) {
	// Create test comparison with files
	comparison := &types.Comparison{
		RepoURL: "https://github.com/test/repo",
		Files: []types.FileChange{
			{
				Filename:  "README.md",
				Patch:     strings.Repeat("line\n", 200), // Large file
				Additions: 200,
				Deletions: 0,
				Status:    "modified",
			},
			{
				Filename:  "small.txt",
				Patch:     "small content\n",
				Additions: 1,
				Deletions: 0,
				Status:    "added",
			},
		},
		Stats: types.ComparisonStats{
			TotalAdditions: 201,
			TotalDeletions: 0,
		},
	}

	t.Run("truncates large files", func(t *testing.T) {
		result, metadata := TruncateMultipleComparisons([]*types.Comparison{comparison}, LevelHigh)

		if len(result) != 1 {
			t.Fatalf("Expected 1 comparison, got %d", len(result))
		}

		if !metadata.Truncated {
			t.Error("Expected truncation to be applied")
		}

		if metadata.Level != LevelHigh {
			t.Errorf("Expected level %s, got %s", LevelHigh, metadata.Level)
		}

		if metadata.FilesTruncated == 0 {
			t.Error("Expected at least one file to be truncated")
		}
	})

	t.Run("nil comparison", func(t *testing.T) {
		result, _ := TruncateMultipleComparisons([]*types.Comparison{nil}, LevelLow)
		if len(result) != 1 || result[0] != nil {
			t.Error("Expected nil comparison to remain nil")
		}
	})
}

func TestCombineMetadata(t *testing.T) {
	metadata1 := &TruncationMetadata{
		Truncated:          true,
		Level:              LevelModerate,
		TotalFiles:         10,
		FilesPreserved:     7,
		FilesTruncated:     3,
		TruncatedFilesList: []string{"file1.go", "file2.go"},
	}

	metadata2 := &TruncationMetadata{
		Truncated:          false,
		Level:              LevelModerate,
		TotalFiles:         5,
		FilesPreserved:     5,
		FilesTruncated:     0,
		TruncatedFilesList: []string{},
	}

	t.Run("combines multiple metadata", func(t *testing.T) {
		result := combineMetadata([]*TruncationMetadata{metadata1, metadata2}, LevelModerate)

		if !result.Truncated {
			t.Error("Expected combined metadata to show truncation")
		}

		if result.TotalFiles != 15 {
			t.Errorf("Expected total files 15, got %d", result.TotalFiles)
		}

		if result.FilesPreserved != 12 {
			t.Errorf("Expected preserved files 12, got %d", result.FilesPreserved)
		}

		if result.FilesTruncated != 3 {
			t.Errorf("Expected truncated files 3, got %d", result.FilesTruncated)
		}

		if len(result.TruncatedFilesList) != 2 {
			t.Errorf("Expected 2 truncated files in list, got %d", len(result.TruncatedFilesList))
		}
	})

	t.Run("handles nil metadata", func(t *testing.T) {
		result := combineMetadata([]*TruncationMetadata{nil, metadata1, nil}, LevelModerate)

		if result.TotalFiles != metadata1.TotalFiles {
			t.Error("Nil metadata should be skipped")
		}
	})
}
