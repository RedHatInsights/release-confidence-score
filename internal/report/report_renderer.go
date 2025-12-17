package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
	"release-confidence-score/internal/llm/truncation"
)

//go:embed report_template.md
var reportTemplateText string

var reportTemplate *template.Template

func init() {
	reportTemplate = template.Must(
		template.New("report").Funcs(templateFuncs()).Parse(reportTemplateText),
	)
}

// templateFuncs returns all custom template functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"hasPrefix":           strings.HasPrefix,
		"contains":            strings.Contains,
		"escapePipes":         escapePipes,
		"qeStatus":            qeStatus,
		"authorizationStatus": authorizationStatus,
		"prLink":              prLink,
		"formatAuthor":        formatAuthor,
		"docURL":              docURL,
		"commitLink":          commitLink,
		"formatDate":          formatDate,
		"docFileInfo":         docFileInfo,
	}
}

// Template helper functions

func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func qeStatus(label string) string {
	switch label {
	case shared.LabelQETested:
		return "✅ Tested"
	case shared.LabelNeedsQETesting:
		return "⚠️ Needs Testing"
	default:
		return "N/A"
	}
}

func authorizationStatus(isAuthorized bool) string {
	if isAuthorized {
		return "✅ Authorized"
	}
	return "❌ Unauthorized"
}

func prLink(prNumber int64, repoURL string) string {
	if prNumber <= 0 {
		return "N/A"
	}

	// GitLab uses /-/merge_requests/, GitHub uses /pull/
	if strings.Contains(repoURL, "github.com") {
		return fmt.Sprintf("[#%d](%s/pull/%d)", prNumber, repoURL, prNumber)
	}
	return fmt.Sprintf("[!%d](%s/-/merge_requests/%d)", prNumber, repoURL, prNumber)
}

func formatAuthor(author, commentURL string) string {
	if strings.Contains(commentURL, "github.com") {
		return fmt.Sprintf("[@%s](https://github.com/%s)", author, author)
	}
	return "@" + author
}

func docURL(filename, repoURL, branch string) string {
	if strings.HasPrefix(filename, "http") {
		return filename
	}
	return fmt.Sprintf("%s/blob/%s/%s", repoURL, branch, filename)
}

func commitLink(shortSHA, fullSHA, repoURL string) string {
	return fmt.Sprintf("[%s](%s/commit/%s)", shortSHA, repoURL, fullSHA)
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

func docFileInfo(filename, repoURL, branch, content string) string {
	url := docURL(filename, repoURL, branch)
	return fmt.Sprintf("- %s - %d chars", url, len(content))
}

// stripMarkdownCodeBlocks removes markdown code block markers from LLM responses
// Handles both ```json and ``` style code blocks
func stripMarkdownCodeBlocks(content string) string {
	trimmed := strings.TrimSpace(content)

	// Return as-is if not wrapped in code blocks
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	// Remove opening marker (```json or ``` followed by newline)
	if idx := strings.Index(trimmed, "\n"); idx != -1 {
		trimmed = trimmed[idx+1:]
	}

	// Remove closing marker
	trimmed = strings.TrimSuffix(trimmed, "```")

	return strings.TrimSpace(trimmed)
}

func getReleaseRecommendation(score, autoDeployThreshold, reviewRequiredThreshold int) string {
	if score >= autoDeployThreshold {
		return "✅ RECOMMENDED FOR RELEASE"
	} else if score >= reviewRequiredThreshold {
		return "⚠️ MANUAL REVIEW REQUIRED"
	} else {
		return "🚫 RELEASE NOT RECOMMENDED"
	}
}

// StructuredAnalysis represents the LLM's analysis output in a structured format
type StructuredAnalysis struct {
	Score                        int               `json:"score"`
	SystemImpactVisual           string            `json:"system_impact_visual"`
	ChangeCharacteristicsVisual  string            `json:"change_characteristics_visual"`
	ActionItems                  ActionItems       `json:"action_items"`
	CodeAnalysis                 TechnicalAnalysis `json:"code_analysis"`
	InfrastructureAnalysis       TechnicalAnalysis `json:"infrastructure_analysis"`
	DependencyAnalysis           TechnicalAnalysis `json:"dependency_analysis"`
	PositiveFactors              string            `json:"positive_factors"`
	RiskFactors                  string            `json:"risk_factors"`
	BlockingIssues               string            `json:"blocking_issues"`
	DocumentationQuality         string            `json:"documentation_quality"`
	DocumentationRecommendations string            `json:"documentation_recommendations"`
}

// ActionItems represents categorized action items
type ActionItems struct {
	Critical  []string `json:"critical"`
	Important []string `json:"important"`
	Followup  []string `json:"followup"`
}

// TechnicalAnalysis represents detailed technical analysis with structured facts
type TechnicalAnalysis struct {
	Summary     string   `json:"summary"`
	KeyFindings []string `json:"key_findings"`
	RiskFactors []string `json:"risk_factors"`
}

// ReportMetadata contains metadata for template replacement
type ReportMetadata struct {
	ModelID        string
	GenerationTime time.Time
}

// ReportConfig holds all configuration and data needed for report generation
type ReportConfig struct {
	LLMResponse             string
	Metadata                *ReportMetadata
	Comparisons             []*types.Comparison
	Documentation           []*types.Documentation
	UserGuidance            []types.UserGuidance
	TruncationInfo          *truncation.TruncationMetadata
	AutoDeployThreshold     int
	ReviewRequiredThreshold int
}

// TemplateData holds all data needed for template rendering
type TemplateData struct {
	Analysis              *StructuredAnalysis
	Metadata              *ReportMetadata
	Comparisons           []*types.Comparison
	Documentation         []*types.Documentation
	ReleaseRecommendation string
	AllUserGuidance       []types.UserGuidance           // All user guidance for comprehensive reporting
	TruncationInfo        *truncation.TruncationMetadata // Optional truncation information
}

// GenerateReport parses LLM response and generates the final report
func GenerateReport(config *ReportConfig) (score int, report string, err error) {
	// Strip markdown code blocks if present (LLMs sometimes wrap JSON in ```json ... ```)
	jsonContent := stripMarkdownCodeBlocks(config.LLMResponse)

	// Parse the structured JSON response
	var analysis StructuredAnalysis
	if err := json.Unmarshal([]byte(jsonContent), &analysis); err != nil {
		return 0, "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Sort user guidance by date (ascending)
	sort.Slice(config.UserGuidance, func(i, j int) bool {
		return config.UserGuidance[i].Date.Before(config.UserGuidance[j].Date)
	})

	// Determine release recommendation based on score
	recommendation := getReleaseRecommendation(analysis.Score, config.AutoDeployThreshold, config.ReviewRequiredThreshold)

	// Create template data
	templateData := &TemplateData{
		Analysis:              &analysis,
		Metadata:              config.Metadata,
		Comparisons:           config.Comparisons,
		Documentation:         config.Documentation,
		ReleaseRecommendation: recommendation,
		AllUserGuidance:       config.UserGuidance,
		TruncationInfo:        config.TruncationInfo,
	}

	// Execute pre-compiled template
	var buf bytes.Buffer
	if err := reportTemplate.Execute(&buf, templateData); err != nil {
		return 0, "", fmt.Errorf("failed to execute report template: %w", err)
	}

	return analysis.Score, buf.String(), nil
}
