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

	"release-confidence-score/internal/changelog"
	githubpkg "release-confidence-score/internal/git/github"
	"release-confidence-score/internal/shared"
)

//go:embed report_template.md
var reportTemplate string

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

// TemplateData holds all data needed for template rendering
type TemplateData struct {
	Analysis              *StructuredAnalysis
	Metadata              *ReportMetadata
	ChangelogContent      string
	Documentation         []*githubpkg.RepoDocumentation
	ReleaseRecommendation string
	AllUserGuidance       []shared.UserGuidance      // All user guidance for comprehensive reporting
	TruncationInfo        *shared.TruncationMetadata // Optional truncation information
}

// GetReleaseRecommendation determines recommendation based on score and thresholds
func (m *ReportMetadata) GetReleaseRecommendation(score int, autoDeployThreshold, reviewRequiredThreshold int) string {
	if score >= autoDeployThreshold {
		return "✅ RECOMMENDED FOR RELEASE"
	} else if score >= reviewRequiredThreshold {
		return "⚠️ MANUAL REVIEW REQUIRED"
	} else {
		return "🚫 RELEASE NOT RECOMMENDED"
	}
}

// ProcessAnalysis converts structured analysis and metadata into final report
func ProcessAnalysis(
	analysis *StructuredAnalysis,
	metadata *ReportMetadata,
	changelogs []*changelog.Changelog,
	documentation []*githubpkg.RepoDocumentation,
	allUserGuidance []shared.UserGuidance,
	truncationInfo *shared.TruncationMetadata,
	autoDeployThreshold, reviewRequiredThreshold int,
) (string, error) {
	// Sort all user guidance by date (ascending)
	// Note: allUserGuidance already contains both GitLab and GitHub guidance
	sort.Slice(allUserGuidance, func(i, j int) bool {
		return allUserGuidance[i].Date.Before(allUserGuidance[j].Date)
	})

	// Create template data
	templateData := &TemplateData{
		Analysis:              analysis,
		Metadata:              metadata,
		ChangelogContent:      changelog.FormatChangelog(changelogs),
		Documentation:         documentation,
		ReleaseRecommendation: metadata.GetReleaseRecommendation(analysis.Score, autoDeployThreshold, reviewRequiredThreshold),
		AllUserGuidance:       allUserGuidance,
		TruncationInfo:        truncationInfo,
	}

	// Parse and execute template
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"hasPrefix": strings.HasPrefix,
		"contains":  strings.Contains,
		"replace":   strings.ReplaceAll,
		"gt":        func(a, b int) bool { return a > b },
		"eq":        func(a, b string) bool { return a == b },
		"len": func(v interface{}) int {
			switch s := v.(type) {
			case string:
				return len(s)
			case []string:
				return len(s)
			default:
				return 0
			}
		},
	}).Parse(reportTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse report template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute report template: %w", err)
	}

	return buf.String(), nil
}

// ParseStructuredResponse attempts to parse LLM response as structured JSON
func ParseStructuredResponse(response string) (*StructuredAnalysis, error) {
	// Try to find JSON in the response (LLM might add extra text)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := response[start : end+1]

	var analysis StructuredAnalysis
	err := json.Unmarshal([]byte(jsonStr), &analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &analysis, nil
}
