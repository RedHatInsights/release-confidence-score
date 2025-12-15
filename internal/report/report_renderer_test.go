package report

import (
	"strings"
	"testing"
	"time"

	"release-confidence-score/internal/git/types"
	"release-confidence-score/internal/llm/truncation"
)

// Test utility functions

func TestStripMarkdownCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no code blocks",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "plain code block",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "with whitespace",
			input:    "  ```json\n{\"key\": \"value\"}\n```  ",
			expected: `{"key": "value"}`,
		},
		{
			name:     "multiline json",
			input:    "```json\n{\n  \"key\": \"value\",\n  \"other\": 123\n}\n```",
			expected: "{\n  \"key\": \"value\",\n  \"other\": 123\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripMarkdownCodeBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("stripMarkdownCodeBlocks() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test template helper functions

func TestEscapePipes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no pipes", "hello world", "hello world"},
		{"single pipe", "hello|world", "hello\\|world"},
		{"multiple pipes", "a|b|c", "a\\|b\\|c"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapePipes(tt.input)
			if result != tt.expected {
				t.Errorf("escapePipes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestQEStatus(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected string
	}{
		{"qe-tested", "qe-tested", "✅ Tested"},
		{"needs-qe-testing", "needs-qe-testing", "⚠️ Needs Testing"},
		{"empty", "", "N/A"},
		{"unknown", "some-other-label", "N/A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qeStatus(tt.label)
			if result != tt.expected {
				t.Errorf("qeStatus(%q) = %q, want %q", tt.label, result, tt.expected)
			}
		})
	}
}

func TestAuthorizationStatus(t *testing.T) {
	tests := []struct {
		name         string
		isAuthorized bool
		expected     string
	}{
		{"authorized", true, "✅ Authorized"},
		{"unauthorized", false, "❌ Unauthorized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := authorizationStatus(tt.isAuthorized)
			if result != tt.expected {
				t.Errorf("authorizationStatus(%v) = %q, want %q", tt.isAuthorized, result, tt.expected)
			}
		})
	}
}

func TestPRLink(t *testing.T) {
	tests := []struct {
		name     string
		prNumber int
		repoURL  string
		expected string
	}{
		{"valid PR", 123, "https://github.com/user/repo", "[#123](https://github.com/user/repo/pull/123)"},
		{"zero PR", 0, "https://github.com/user/repo", "N/A"},
		{"negative PR", -1, "https://github.com/user/repo", "N/A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prLink(tt.prNumber, tt.repoURL)
			if result != tt.expected {
				t.Errorf("prLink(%d, %q) = %q, want %q", tt.prNumber, tt.repoURL, result, tt.expected)
			}
		})
	}
}

func TestFormatAuthor(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		commentURL string
		expected   string
	}{
		{"github user", "johndoe", "https://github.com/owner/repo/pull/1#comment", "[@johndoe](https://github.com/johndoe)"},
		{"gitlab user", "janedoe", "https://gitlab.com/owner/repo/-/merge_requests/1#note", "@janedoe"},
		{"other platform", "user", "https://example.com/comment/1", "@user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAuthor(tt.author, tt.commentURL)
			if result != tt.expected {
				t.Errorf("formatAuthor(%q, %q) = %q, want %q", tt.author, tt.commentURL, result, tt.expected)
			}
		})
	}
}

func TestDocURL(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		repoURL  string
		branch   string
		expected string
	}{
		{
			"relative path",
			"README.md",
			"https://github.com/user/repo",
			"main",
			"https://github.com/user/repo/blob/main/README.md",
		},
		{
			"http URL",
			"http://example.com/doc.md",
			"https://github.com/user/repo",
			"main",
			"http://example.com/doc.md",
		},
		{
			"https URL",
			"https://example.com/doc.md",
			"https://github.com/user/repo",
			"main",
			"https://example.com/doc.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docURL(tt.filename, tt.repoURL, tt.branch)
			if result != tt.expected {
				t.Errorf("docURL(%q, %q, %q) = %q, want %q", tt.filename, tt.repoURL, tt.branch, result, tt.expected)
			}
		})
	}
}

func TestCommitLink(t *testing.T) {
	tests := []struct {
		name     string
		shortSHA string
		fullSHA  string
		repoURL  string
		expected string
	}{
		{
			"standard commit",
			"abc123",
			"abc123def456",
			"https://github.com/user/repo",
			"[abc123](https://github.com/user/repo/commit/abc123def456)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := commitLink(tt.shortSHA, tt.fullSHA, tt.repoURL)
			if result != tt.expected {
				t.Errorf("commitLink(%q, %q, %q) = %q, want %q", tt.shortSHA, tt.fullSHA, tt.repoURL, result, tt.expected)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			"standard date",
			time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			"2024-01-15 14:30",
		},
		{
			"zero time",
			time.Time{},
			"0001-01-01 00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDate(tt.input)
			if result != tt.expected {
				t.Errorf("formatDate(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDocFileInfo(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		repoURL  string
		branch   string
		content  string
		expected string
	}{
		{
			"standard file",
			"README.md",
			"https://github.com/user/repo",
			"main",
			"This is content",
			"- https://github.com/user/repo/blob/main/README.md - 15 chars",
		},
		{
			"external URL",
			"https://example.com/doc.md",
			"https://github.com/user/repo",
			"main",
			"Content",
			"- https://example.com/doc.md - 7 chars",
		},
		{
			"empty content",
			"empty.md",
			"https://github.com/user/repo",
			"main",
			"",
			"- https://github.com/user/repo/blob/main/empty.md - 0 chars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docFileInfo(tt.filename, tt.repoURL, tt.branch, tt.content)
			if result != tt.expected {
				t.Errorf("docFileInfo(%q, %q, %q, %q) = %q, want %q", tt.filename, tt.repoURL, tt.branch, tt.content, result, tt.expected)
			}
		})
	}
}

func TestGetReleaseRecommendation(t *testing.T) {
	tests := []struct {
		name                    string
		score                   int
		autoDeployThreshold     int
		reviewRequiredThreshold int
		expected                string
	}{
		{
			"auto deploy",
			90,
			80,
			60,
			"✅ RECOMMENDED FOR RELEASE",
		},
		{
			"at auto deploy threshold",
			80,
			80,
			60,
			"✅ RECOMMENDED FOR RELEASE",
		},
		{
			"manual review",
			70,
			80,
			60,
			"⚠️ MANUAL REVIEW REQUIRED",
		},
		{
			"at review threshold",
			60,
			80,
			60,
			"⚠️ MANUAL REVIEW REQUIRED",
		},
		{
			"not recommended",
			50,
			80,
			60,
			"🚫 RELEASE NOT RECOMMENDED",
		},
		{
			"very low score",
			10,
			80,
			60,
			"🚫 RELEASE NOT RECOMMENDED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getReleaseRecommendation(tt.score, tt.autoDeployThreshold, tt.reviewRequiredThreshold)
			if result != tt.expected {
				t.Errorf("getReleaseRecommendation(%d, %d, %d) = %q, want %q", tt.score, tt.autoDeployThreshold, tt.reviewRequiredThreshold, result, tt.expected)
			}
		})
	}
}

func TestTemplateFuncs(t *testing.T) {
	funcs := templateFuncs()

	expectedFuncs := []string{
		"hasPrefix",
		"contains",
		"escapePipes",
		"qeStatus",
		"authorizationStatus",
		"prLink",
		"formatAuthor",
		"docURL",
		"commitLink",
		"formatDate",
		"docFileInfo",
	}

	for _, name := range expectedFuncs {
		if _, ok := funcs[name]; !ok {
			t.Errorf("templateFuncs() missing expected function: %s", name)
		}
	}
}

func TestGenerateReport(t *testing.T) {
	// Test with minimal valid JSON
	minimalJSON := `{
		"score": 85,
		"system_impact_visual": "Low impact",
		"change_characteristics_visual": "Bug fix",
		"action_items": {
			"critical": [],
			"important": [],
			"followup": []
		},
		"code_analysis": {
			"summary": "Code looks good",
			"key_findings": [],
			"risk_factors": []
		},
		"infrastructure_analysis": {
			"summary": "No infrastructure changes",
			"key_findings": [],
			"risk_factors": []
		},
		"dependency_analysis": {
			"summary": "Dependencies updated",
			"key_findings": [],
			"risk_factors": []
		},
		"positive_factors": "Well tested",
		"risk_factors": "None",
		"blocking_issues": "",
		"documentation_quality": "Good",
		"documentation_recommendations": "None"
	}`

	config := &ReportConfig{
		LLMResponse:             minimalJSON,
		Metadata:                &ReportMetadata{ModelID: "test-model", GenerationTime: time.Now()},
		Comparisons:             nil,
		Documentation:           nil,
		UserGuidance:            nil,
		TruncationInfo:          nil,
		AutoDeployThreshold:     80,
		ReviewRequiredThreshold: 60,
	}

	score, report, err := GenerateReport(config)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if score != 85 {
		t.Errorf("GenerateReport() score = %d, want 85", score)
	}

	if report == "" {
		t.Error("GenerateReport() returned empty report")
	}

	// Check that the report contains expected sections
	expectedSections := []string{
		"Release Confidence Report",
		"Confidence Score: **85/100**",
		"✅ RECOMMENDED FOR RELEASE",
		"Technical Analysis",
		"Code Impact Analysis",
		"Infrastructure & Deployment Impact",
		"Dependencies & Integration Impact",
	}

	for _, section := range expectedSections {
		if !strings.Contains(report, section) {
			t.Errorf("GenerateReport() report missing section: %q", section)
		}
	}
}

func TestGenerateReportInvalidJSON(t *testing.T) {
	config := &ReportConfig{
		LLMResponse:             "not valid json",
		Metadata:                &ReportMetadata{ModelID: "test-model", GenerationTime: time.Now()},
		AutoDeployThreshold:     80,
		ReviewRequiredThreshold: 60,
	}

	_, _, err := GenerateReport(config)
	if err == nil {
		t.Error("GenerateReport() expected error for invalid JSON, got nil")
	}
}

func TestGenerateReportWithUserGuidance(t *testing.T) {
	jsonResponse := `{
		"score": 75,
		"system_impact_visual": "Medium impact",
		"change_characteristics_visual": "Feature addition",
		"action_items": {
			"critical": ["Test thoroughly"],
			"important": ["Update docs"],
			"followup": []
		},
		"code_analysis": {
			"summary": "New feature added",
			"key_findings": ["Clean code"],
			"risk_factors": ["Needs testing"]
		},
		"infrastructure_analysis": {
			"summary": "No changes",
			"key_findings": [],
			"risk_factors": []
		},
		"dependency_analysis": {
			"summary": "Dependencies updated",
			"key_findings": [],
			"risk_factors": []
		},
		"positive_factors": "Well structured",
		"risk_factors": "Limited testing",
		"blocking_issues": "",
		"documentation_quality": "Adequate",
		"documentation_recommendations": "Add examples"
	}`

	// Create user guidance with different dates to test sorting
	userGuidance := []types.UserGuidance{
		{
			Content:      "Third guidance",
			Author:       "user3",
			Date:         time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
			CommentURL:   "https://github.com/owner/repo/pull/1#comment3",
			IsAuthorized: true,
		},
		{
			Content:      "First guidance",
			Author:       "user1",
			Date:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			CommentURL:   "https://github.com/owner/repo/pull/1#comment1",
			IsAuthorized: true,
		},
		{
			Content:      "Second guidance",
			Author:       "user2",
			Date:         time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			CommentURL:   "https://gitlab.com/owner/repo/-/merge_requests/1#note",
			IsAuthorized: false,
		},
	}

	config := &ReportConfig{
		LLMResponse:             jsonResponse,
		Metadata:                &ReportMetadata{ModelID: "test-model", GenerationTime: time.Now()},
		UserGuidance:            userGuidance,
		AutoDeployThreshold:     80,
		ReviewRequiredThreshold: 60,
	}

	score, report, err := GenerateReport(config)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if score != 75 {
		t.Errorf("GenerateReport() score = %d, want 75", score)
	}

	// Verify user guidance section exists
	if !strings.Contains(report, "User Guidance") {
		t.Error("GenerateReport() report missing User Guidance section")
	}

	// Verify guidance is included
	if !strings.Contains(report, "First guidance") {
		t.Error("GenerateReport() report missing first guidance")
	}
	if !strings.Contains(report, "Second guidance") {
		t.Error("GenerateReport() report missing second guidance")
	}
	if !strings.Contains(report, "Third guidance") {
		t.Error("GenerateReport() report missing third guidance")
	}

	// Verify authorization status
	if !strings.Contains(report, "✅ Authorized") {
		t.Error("GenerateReport() report missing authorized status")
	}
	if !strings.Contains(report, "❌ Unauthorized") {
		t.Error("GenerateReport() report missing unauthorized status")
	}
}

func TestGenerateReportWithComparisons(t *testing.T) {
	jsonResponse := `{
		"score": 80,
		"system_impact_visual": "Low impact",
		"change_characteristics_visual": "Bug fix",
		"action_items": {"critical": [], "important": [], "followup": []},
		"code_analysis": {"summary": "Good", "key_findings": [], "risk_factors": []},
		"infrastructure_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"dependency_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"positive_factors": "Tested",
		"risk_factors": "None",
		"blocking_issues": "",
		"documentation_quality": "Good",
		"documentation_recommendations": "None"
	}`

	comparisons := []*types.Comparison{
		{
			RepoURL: "https://github.com/user/repo",
			DiffURL: "https://github.com/user/repo/compare/v1...v2",
			Commits: []types.Commit{
				{
					SHA:            "abc123def456",
					ShortSHA:       "abc123",
					Message:        "Fix bug | with pipe",
					Author:         "John Doe",
					PRNumber:       123,
					QETestingLabel: "qe-tested",
				},
				{
					SHA:            "def456abc789",
					ShortSHA:       "def456",
					Message:        "Another fix",
					Author:         "Jane Smith",
					PRNumber:       0,
					QETestingLabel: "needs-qe-testing",
				},
			},
			Files: []types.FileChange{},
			Stats: types.ComparisonStats{},
		},
	}

	config := &ReportConfig{
		LLMResponse:             jsonResponse,
		Metadata:                &ReportMetadata{ModelID: "test-model", GenerationTime: time.Now()},
		Comparisons:             comparisons,
		AutoDeployThreshold:     80,
		ReviewRequiredThreshold: 60,
	}

	score, report, err := GenerateReport(config)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if score != 80 {
		t.Errorf("GenerateReport() score = %d, want 80", score)
	}

	// Verify changelog section
	if !strings.Contains(report, "Release Changelogs") {
		t.Error("GenerateReport() report missing Release Changelogs section")
	}

	// Verify commits are included
	if !strings.Contains(report, "abc123") {
		t.Error("GenerateReport() report missing first commit")
	}
	if !strings.Contains(report, "def456") {
		t.Error("GenerateReport() report missing second commit")
	}

	// Verify pipe escaping
	if !strings.Contains(report, "Fix bug \\| with pipe") {
		t.Error("GenerateReport() report did not escape pipes in commit message")
	}

	// Verify QE status
	if !strings.Contains(report, "✅ Tested") {
		t.Error("GenerateReport() report missing QE tested status")
	}
	if !strings.Contains(report, "⚠️ Needs Testing") {
		t.Error("GenerateReport() report missing QE needs testing status")
	}

	// Verify PR link
	if !strings.Contains(report, "#123") {
		t.Error("GenerateReport() report missing PR link")
	}
	if !strings.Contains(report, "N/A") {
		t.Error("GenerateReport() report missing N/A for commit without PR")
	}
}

func TestGenerateReportWithDocumentation(t *testing.T) {
	jsonResponse := `{
		"score": 90,
		"system_impact_visual": "Low impact",
		"change_characteristics_visual": "Documentation",
		"action_items": {"critical": [], "important": [], "followup": []},
		"code_analysis": {"summary": "Good", "key_findings": [], "risk_factors": []},
		"infrastructure_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"dependency_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"positive_factors": "Well documented",
		"risk_factors": "None",
		"blocking_issues": "",
		"documentation_quality": "Excellent",
		"documentation_recommendations": "Keep it up"
	}`

	docs := []*types.Documentation{
		{
			Repository: types.Repository{
				URL:           "https://github.com/user/repo",
				DefaultBranch: "main",
			},
			MainDocFile:         "README.md",
			MainDocContent:      "# Project\n\nDescription",
			AdditionalDocsOrder: []string{"CONTRIBUTING.md", "https://example.com/external-doc.md"},
			AdditionalDocsContent: map[string]string{
				"CONTRIBUTING.md":                     "Contribution guidelines",
				"https://example.com/external-doc.md": "External documentation",
			},
		},
	}

	config := &ReportConfig{
		LLMResponse:             jsonResponse,
		Metadata:                &ReportMetadata{ModelID: "test-model", GenerationTime: time.Now()},
		Documentation:           docs,
		AutoDeployThreshold:     80,
		ReviewRequiredThreshold: 60,
	}

	score, report, err := GenerateReport(config)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if score != 90 {
		t.Errorf("GenerateReport() score = %d, want 90", score)
	}

	// Verify documentation section
	if !strings.Contains(report, "Documentation Sources Analyzed") {
		t.Error("GenerateReport() report missing Documentation Sources section")
	}

	// Verify files are listed
	if !strings.Contains(report, "README.md") {
		t.Error("GenerateReport() report missing README.md")
	}
	if !strings.Contains(report, "CONTRIBUTING.md") {
		t.Error("GenerateReport() report missing CONTRIBUTING.md")
	}

	// Verify external URL is preserved
	if !strings.Contains(report, "https://example.com/external-doc.md") {
		t.Error("GenerateReport() report missing external doc URL")
	}

	// Verify char counts
	if !strings.Contains(report, "chars") {
		t.Error("GenerateReport() report missing char counts")
	}
}

func TestGenerateReportWithTruncation(t *testing.T) {
	jsonResponse := `{
		"score": 70,
		"system_impact_visual": "Medium impact",
		"change_characteristics_visual": "Large change",
		"action_items": {"critical": [], "important": [], "followup": []},
		"code_analysis": {"summary": "Good", "key_findings": [], "risk_factors": []},
		"infrastructure_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"dependency_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"positive_factors": "Structured",
		"risk_factors": "Large diff",
		"blocking_issues": "",
		"documentation_quality": "Good",
		"documentation_recommendations": "None"
	}`

	truncationInfo := &truncation.TruncationMetadata{
		Level:          "moderate",
		TotalFiles:     100,
		FilesPreserved: 80,
		FilesTruncated: 20,
	}

	config := &ReportConfig{
		LLMResponse:             jsonResponse,
		Metadata:                &ReportMetadata{ModelID: "test-model", GenerationTime: time.Now()},
		TruncationInfo:          truncationInfo,
		AutoDeployThreshold:     80,
		ReviewRequiredThreshold: 60,
	}

	score, report, err := GenerateReport(config)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if score != 70 {
		t.Errorf("GenerateReport() score = %d, want 70", score)
	}

	// Verify truncation warning
	if !strings.Contains(report, "Diff Truncation Applied") {
		t.Error("GenerateReport() report missing truncation warning")
	}
	if !strings.Contains(report, "moderate") {
		t.Error("GenerateReport() report missing truncation level")
	}
	if !strings.Contains(report, "80/100") {
		t.Error("GenerateReport() report missing files preserved count")
	}
}

func TestGenerateReportWithAggressiveTruncation(t *testing.T) {
	jsonResponse := `{
		"score": 65,
		"system_impact_visual": "High impact",
		"change_characteristics_visual": "Very large change",
		"action_items": {"critical": [], "important": [], "followup": []},
		"code_analysis": {"summary": "Good", "key_findings": [], "risk_factors": []},
		"infrastructure_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"dependency_analysis": {"summary": "None", "key_findings": [], "risk_factors": []},
		"positive_factors": "Structured",
		"risk_factors": "Very large diff",
		"blocking_issues": "",
		"documentation_quality": "Good",
		"documentation_recommendations": "None"
	}`

	truncationInfo := &truncation.TruncationMetadata{
		Level:          "aggressive",
		TotalFiles:     200,
		FilesPreserved: 50,
		FilesTruncated: 150,
	}

	config := &ReportConfig{
		LLMResponse:             jsonResponse,
		Metadata:                &ReportMetadata{ModelID: "test-model", GenerationTime: time.Now()},
		TruncationInfo:          truncationInfo,
		AutoDeployThreshold:     80,
		ReviewRequiredThreshold: 60,
	}

	score, report, err := GenerateReport(config)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if score != 65 {
		t.Errorf("GenerateReport() score = %d, want 65", score)
	}

	// Verify aggressive truncation level triggers additional message
	if !strings.Contains(report, "aggressive") {
		t.Error("GenerateReport() report missing aggressive truncation level")
	}
	if !strings.Contains(report, "Middle sections of medium-risk files") {
		t.Error("GenerateReport() report missing aggressive truncation details")
	}
}
