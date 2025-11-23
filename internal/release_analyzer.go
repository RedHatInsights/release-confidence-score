package internal

import (
	"fmt"
	"log/slog"
	"time"

	"release-confidence-score/internal/app_interface"
	"release-confidence-score/internal/changelog"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/github"
	"release-confidence-score/internal/git/gitlab"
	"release-confidence-score/internal/llm"
	"release-confidence-score/internal/llm/providers"
	"release-confidence-score/internal/qe"
	"release-confidence-score/internal/report"
	"release-confidence-score/internal/shared"

	githubapi "github.com/google/go-github/v79/github"
	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type ReleaseAnalyzer struct {
	githubClient *githubapi.Client
	gitlabClient *gitlabapi.Client
	llmClient    llm.LLMClient
	config       *config.Config
}

func New(cfg *config.Config) (*ReleaseAnalyzer, error) {

	githubClient := github.NewClient(cfg)
	gitlabClient := gitlab.NewClient(cfg)

	llmClient, err := providers.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &ReleaseAnalyzer{
		githubClient: githubClient,
		gitlabClient: gitlabClient,
		llmClient:    llmClient,
		config:       cfg,
	}, nil
}

func (ra *ReleaseAnalyzer) AnalyzeAppInterface(mergeRequestIID int) (float64, string, error) {
	slog.Debug("Starting release analysis in app-interface mode")

	// Phase 1: Fetch all data
	// Get diff URLs and user guidance from merge request notes
	diffURLs, appInterfaceGuidance, err := app_interface.GetDiffURLsAndUserGuidance(ra.gitlabClient, ra.config, mergeRequestIID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get release data from app-interface: %w", err)
	}

	// Fetch raw release data from GitHub
	changelogs, githubGuidance, documentation, comparisons, err := GetReleaseData(ra.githubClient, ra.config, diffURLs)
	if err != nil {
		return 0, "", fmt.Errorf("failed to fetch release data: %w", err)
	}

	// Merge user guidance from GitLab and GitHub sources
	allUserGuidance := make([]shared.UserGuidance, 0, len(appInterfaceGuidance)+len(githubGuidance))
	allUserGuidance = append(allUserGuidance, appInterfaceGuidance...)
	allUserGuidance = append(allUserGuidance, githubGuidance...)

	return ra.analyze(changelogs, allUserGuidance, documentation, comparisons)
}

// AnalyzeStandalone performs release analysis using compare URLs directly (standalone mode)
func (ra *ReleaseAnalyzer) AnalyzeStandalone(compareURLs []string) (float64, string, error) {
	slog.Debug("Starting release analysis in standalone mode", "url_count", len(compareURLs))

	if len(compareURLs) == 0 {
		return 0, "", fmt.Errorf("no compare URLs provided")
	}

	// Fetch raw release data from GitHub
	changelogs, githubGuidance, documentation, comparisons, err := GetReleaseData(ra.githubClient, ra.config, compareURLs)
	if err != nil {
		return 0, "", fmt.Errorf("failed to fetch release data: %w", err)
	}

	return ra.analyze(changelogs, githubGuidance, documentation, comparisons)
}

// analyzeRelease is the common analysis logic used by both modes
func (ra *ReleaseAnalyzer) analyze(
	changelogs []*changelog.Changelog,
	userGuidance []shared.UserGuidance,
	documentation []*github.RepoDocumentation,
	comparisons []*github.CompareData,
) (float64, string, error) {
	// Process QE testing labels from changelogs
	qeTestingCommits := qe.BuildTestingCommits(changelogs)

	// Phases 2-4: Format data, submit prompt, and handle truncation retries
	// This is handled internally by AnalyzeWithProgressiveTruncation
	response, truncationInfo, err := llm.AnalyzeWithProgressiveTruncation(
		ra.llmClient, comparisons, documentation, userGuidance, qeTestingCommits)
	if err != nil {
		return 0, "", err
	}

	// Phase 5: Render template and return report and score
	// Parse the structured JSON response
	analysis, err := report.ParseStructuredResponse(response)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse structured response: %w", err)
	}

	// Build metadata for template processing
	reportMetadata := &report.ReportMetadata{
		ModelID:        ra.config.ModelID,
		GenerationTime: time.Now(),
	}

	// Process the analysis into final report using embedded template
	finalReport, err := report.ProcessAnalysis(analysis, reportMetadata, changelogs, documentation, userGuidance,
		truncationInfo, ra.config.ScoreThresholds.AutoDeploy, ra.config.ScoreThresholds.ReviewRequired)
	if err != nil {
		return 0, "", fmt.Errorf("failed to process analysis into report: %w", err)
	}

	// Return the structured score and processed report
	return float64(analysis.Score), finalReport, nil
}
