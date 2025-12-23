package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"release-confidence-score/internal/app_interface"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/github"
	"release-confidence-score/internal/git/gitlab"
	"release-confidence-score/internal/git/types"
	llmerrors "release-confidence-score/internal/llm/errors"
	"release-confidence-score/internal/llm/formatting"
	"release-confidence-score/internal/llm/prompts/user"
	"release-confidence-score/internal/llm/providers"
	"release-confidence-score/internal/llm/truncation"
	"release-confidence-score/internal/report"

	"golang.org/x/sync/errgroup"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type ReleaseAnalyzer struct {
	githubProvider types.GitProvider
	gitlabProvider types.GitProvider
	gitlabClient   *gitlabapi.Client // Still needed for app_interface.GetDiffURLsAndUserGuidance
	llmClient      providers.LLMClient
	config         *config.Config
}

func New(cfg *config.Config) (*ReleaseAnalyzer, error) {
	githubClient := github.NewClient(cfg)

	gitlabClient, err := gitlab.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	llmClient, err := providers.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &ReleaseAnalyzer{
		githubProvider: github.NewFetcher(githubClient, cfg),
		gitlabProvider: gitlab.NewFetcher(gitlabClient, cfg),
		gitlabClient:   gitlabClient, // Keep for app_interface package
		llmClient:      llmClient,
		config:         cfg,
	}, nil
}

func (ra *ReleaseAnalyzer) AnalyzeAppInterface(mergeRequestIID int64, postToMR bool) (float64, string, error) {
	slog.Debug("Starting release analysis in app-interface mode")

	// Get diff URLs and user guidance from merge request notes
	diffURLs, appInterfaceGuidance, err := app_interface.GetDiffURLsAndUserGuidance(ra.gitlabClient, ra.config, mergeRequestIID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get release data from app-interface: %w", err)
	}

	// Fetch raw release data from GitHub and/or GitLab
	comparisons, gitGuidance, documentation, err := ra.getReleaseData(diffURLs)
	if err != nil {
		return 0, "", fmt.Errorf("failed to fetch release data: %w", err)
	}

	// Merge user guidance from app-interface MR and Git sources
	allGuidance := make([]types.UserGuidance, 0, len(appInterfaceGuidance)+len(gitGuidance))
	allGuidance = append(allGuidance, appInterfaceGuidance...)
	allGuidance = append(allGuidance, gitGuidance...)

	score, reportText, err := ra.analyze(comparisons, allGuidance, documentation)
	if err != nil {
		return 0, "", err
	}

	// Post report to MR if requested
	if postToMR {
		if err := app_interface.PostReportToMR(ra.gitlabClient, reportText, mergeRequestIID); err != nil {
			return 0, "", fmt.Errorf("failed to post report to MR: %w", err)
		}
		slog.Info("Report posted to merge request", "mr_iid", mergeRequestIID)
	}

	return score, reportText, nil
}

// AnalyzeStandalone performs release analysis using compare URLs directly (standalone mode)
func (ra *ReleaseAnalyzer) AnalyzeStandalone(compareURLs []string) (float64, string, error) {
	slog.Debug("Starting release analysis in standalone mode", "url_count", len(compareURLs))

	if len(compareURLs) == 0 {
		return 0, "", fmt.Errorf("no compare URLs provided")
	}

	// Fetch raw release data from GitHub and/or GitLab
	comparisons, gitGuidance, documentation, err := ra.getReleaseData(compareURLs)
	if err != nil {
		return 0, "", fmt.Errorf("failed to fetch release data: %w", err)
	}

	return ra.analyze(comparisons, gitGuidance, documentation)
}

// getReleaseData fetches raw release data from multiple compare URLs (GitHub or GitLab)
// URLs are processed in parallel for better performance
// Returns: comparisons, user guidance, documentation, error
func (ra *ReleaseAnalyzer) getReleaseData(urls []string) ([]*types.Comparison, []types.UserGuidance, []*types.Documentation, error) {
	if len(urls) == 0 {
		return []*types.Comparison{}, []types.UserGuidance{}, []*types.Documentation{}, nil
	}

	// Deduplicate URLs while preserving order
	uniqueURLs := make([]string, 0, len(urls))
	seen := make(map[string]bool)
	duplicateCount := 0

	for _, url := range urls {
		if !seen[url] {
			seen[url] = true
			uniqueURLs = append(uniqueURLs, url)
		} else {
			duplicateCount++
		}
	}

	if duplicateCount > 0 {
		slog.Debug("Deduplicated compare URLs", "total", len(urls), "unique", len(uniqueURLs), "duplicates_removed", duplicateCount)
	}

	// Fetch all URLs in parallel
	g, gCtx := errgroup.WithContext(context.Background())

	var mu sync.Mutex // Protects concurrent appends to result slices
	var comparisons []*types.Comparison
	var allUserGuidance []types.UserGuidance
	var documentation []*types.Documentation

	for _, url := range uniqueURLs {
		g.Go(func() error {
			// Detect which provider to use based on URL
			var provider types.GitProvider
			switch {
			case ra.githubProvider.IsCompareURL(url):
				provider = ra.githubProvider
			case ra.gitlabProvider.IsCompareURL(url):
				provider = ra.gitlabProvider
			default:
				slog.Warn("Skipping unsupported URL", "url", url)
				return nil
			}

			platformName := provider.Name()
			slog.Debug("Fetching data", "platform", platformName, "url", url)

			// Fetch all release data (comparison with enriched commits, user guidance, documentation)
			comparison, userGuidance, docs, err := provider.FetchReleaseData(gCtx, url)
			if err != nil {
				slog.Error("Error fetching data", "platform", platformName, "error", err, "url", url)
				return nil
			}

			mu.Lock()
			defer mu.Unlock()

			// Collect comparison if available
			if comparison != nil {
				slog.Debug("Collected comparison",
					"platform", platformName,
					"commit_count", len(comparison.Commits),
					"file_count", len(comparison.Files))
				comparisons = append(comparisons, comparison)
			} else {
				slog.Warn("No comparison data received", "platform", platformName)
			}

			// Collect user guidance
			if len(userGuidance) > 0 {
				slog.Debug("Collected user guidance", "platform", platformName, "count", len(userGuidance))
				allUserGuidance = append(allUserGuidance, userGuidance...)
			}

			// Collect documentation if available
			if docs != nil && docs.MainDocFile != "" {
				slog.Debug("Collected documentation",
					"platform", platformName,
					"repo_url", docs.Repository.URL,
					"entry_point", docs.MainDocFile)
				documentation = append(documentation, docs)
			}

			return nil
		})
	}

	g.Wait()

	return comparisons, allUserGuidance, documentation, nil
}

// analyze is the common analysis logic used by both modes
func (ra *ReleaseAnalyzer) analyze(
	comparisons []*types.Comparison,
	userGuidance []types.UserGuidance,
	documentation []*types.Documentation,
) (float64, string, error) {
	// Analyze with progressive truncation retry
	response, truncationInfo, err := ra.analyzeWithProgressiveTruncation(
		comparisons, documentation, userGuidance)
	if err != nil {
		return 0, "", err
	}

	// Phase 5: Render template and return report and score
	// Build report configuration
	reportConfig := &report.ReportConfig{
		LLMResponse: response,
		Metadata: &report.ReportMetadata{
			ModelID:        ra.config.ModelID,
			GenerationTime: time.Now(),
		},
		Comparisons:             comparisons,
		Documentation:           documentation,
		UserGuidance:            userGuidance,
		TruncationInfo:          truncationInfo,
		AutoDeployThreshold:     ra.config.ScoreThresholds.AutoDeploy,
		ReviewRequiredThreshold: ra.config.ScoreThresholds.ReviewRequired,
	}

	// Generate the final report and extract score
	score, finalReport, err := report.GenerateReport(reportConfig)
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate report: %w", err)
	}

	// Return the structured score and processed report
	return float64(score), finalReport, nil
}

// analyzeWithProgressiveTruncation performs LLM analysis with automatic retry using progressive truncation
// Returns the LLM response, truncation metadata (nil if no truncation), and any error
func (ra *ReleaseAnalyzer) analyzeWithProgressiveTruncation(
	comparisons []*types.Comparison,
	documentation []*types.Documentation,
	allUserGuidance []types.UserGuidance,
) (string, *truncation.TruncationMetadata, error) {
	// Format data and prepare initial prompt
	diffContent := formatting.FormatComparisons(comparisons)
	documentationText := formatting.FormatDocumentations(documentation)

	userPrompt, err := user.RenderUserPrompt(diffContent, documentationText, allUserGuidance, truncation.TruncationMetadata{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to format initial user prompt: %w", err)
	}

	// Try with full diff first
	response, err := ra.llmClient.Analyze(userPrompt)
	if err == nil {
		return response, nil, nil
	}

	// Check if this is a context window error
	contextErr, ok := err.(*llmerrors.ContextWindowError)
	if !ok {
		return "", nil, fmt.Errorf("failed to analyze merge request: %w", err)
	}

	// Retry with progressive truncation
	slog.Warn("Context window exceeded, retrying with progressive truncation",
		"provider", contextErr.Provider,
		"status_code", contextErr.StatusCode)

	truncationLevels := []string{
		truncation.LevelLow,
		truncation.LevelModerate,
		truncation.LevelHigh,
		truncation.LevelExtreme,
	}

	for _, levelName := range truncationLevels {
		slog.Info("Attempting analysis with truncation", "level", levelName)

		// Truncate diffs and documentation at the same level
		truncatedComparisons, combinedMetadata := truncation.TruncateMultipleComparisons(comparisons, levelName)
		truncatedDocs := truncation.TruncateDocumentation(documentation, levelName)

		// Format truncated comparisons and documentation
		truncatedDiff := formatting.FormatComparisons(truncatedComparisons)
		truncatedDocText := formatting.FormatDocumentations(truncatedDocs)

		userPrompt, promptErr := user.RenderUserPrompt(truncatedDiff, truncatedDocText, allUserGuidance, combinedMetadata)
		if promptErr != nil {
			return "", nil, fmt.Errorf("failed to format user prompt with %s truncation: %w", levelName, promptErr)
		}

		// Retry analysis with truncated prompt
		response, err = ra.llmClient.Analyze(userPrompt)
		if err != nil {
			// Check if still a context window error
			if _, isContextErr := err.(*llmerrors.ContextWindowError); isContextErr {
				slog.Warn("Context window still exceeded with truncation", "level", levelName)
				continue
			}
			// Different error - fail immediately
			return "", nil, fmt.Errorf("failed to analyze merge request with %s truncation: %w", levelName, err)
		}

		// Success!
		slog.Info("Analysis succeeded with truncation", "level", levelName)
		return response, &combinedMetadata, nil
	}

	// Exhausted all truncation levels
	return "", nil, fmt.Errorf("failed to analyze merge request even with extreme truncation: %w", err)
}
