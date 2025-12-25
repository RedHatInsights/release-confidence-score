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
)

var truncationLevels = []string{
	truncation.LevelLow,
	truncation.LevelModerate,
	truncation.LevelHigh,
	truncation.LevelExtreme,
}

type ReleaseAnalyzer struct {
	githubProvider types.GitProvider
	gitlabProvider types.GitProvider
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
		llmClient:      llmClient,
		config:         cfg,
	}, nil
}

func (ra *ReleaseAnalyzer) AnalyzeAppInterface(mergeRequestIID int64, postToMR bool) (float64, string, error) {
	slog.Debug("Starting release analysis in app-interface mode")

	// Create GitLab client for app-interface API calls
	gitlabClient, err := gitlab.NewClient(ra.config)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Get diff URLs and user guidance from merge request notes
	diffURLs, appInterfaceGuidance, err := app_interface.GetDiffURLsAndUserGuidance(gitlabClient, ra.config, mergeRequestIID)
	if err != nil {
		return 0, "", err
	}

	// Fetch raw release data from GitHub and/or GitLab
	comparisons, gitGuidance, documentation, err := ra.getReleaseData(diffURLs)
	if err != nil {
		return 0, "", err
	}

	// Merge user guidance from app-interface MR and Git sources
	allGuidance := append(appInterfaceGuidance, gitGuidance...)

	score, reportText, err := ra.analyze(comparisons, allGuidance, documentation)
	if err != nil {
		return 0, "", err
	}

	// Post report to MR if requested
	if postToMR {
		if err := app_interface.PostReportToMR(gitlabClient, reportText, mergeRequestIID); err != nil {
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
		return 0, "", err
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

	// Deduplicate URLs
	seen := make(map[string]bool)
	uniqueURLs := make([]string, 0, len(urls))
	for _, url := range urls {
		if !seen[url] {
			seen[url] = true
			uniqueURLs = append(uniqueURLs, url)
		}
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
				return fmt.Errorf("unsupported compare URL: %s", url)
			}

			slog.Debug("Fetching data", "platform", provider.Name(), "url", url)

			// Fetch all release data (comparison with augmented commits, user guidance, documentation)
			comparison, userGuidance, docs, err := provider.FetchReleaseData(gCtx, url)
			if err != nil {
				return fmt.Errorf("failed to fetch data from %s: %w", url, err)
			}

			if comparison == nil {
				return fmt.Errorf("no comparison data returned for %s", url)
			}

			mu.Lock()
			defer mu.Unlock()

			comparisons = append(comparisons, comparison)

			// Collect user guidance
			if len(userGuidance) > 0 {
				slog.Debug("Collected user guidance", "platform", provider.Name(), "count", len(userGuidance))
				allUserGuidance = append(allUserGuidance, userGuidance...)
			}

			// Collect documentation if available
			if docs != nil && docs.MainDocFile != "" {
				slog.Debug("Collected documentation",
					"platform", provider.Name(),
					"repo_url", docs.Repository.URL,
					"main_doc_file", docs.MainDocFile)
				documentation = append(documentation, docs)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, nil, err
	}

	return comparisons, allUserGuidance, documentation, nil
}

// analyze formats data, calls the LLM (with progressive truncation if needed), and generates the report
func (ra *ReleaseAnalyzer) analyze(comparisons []*types.Comparison, userGuidance []types.UserGuidance, documentation []*types.Documentation) (float64, string, error) {
	// Format data and prepare initial prompt
	diffContent := formatting.FormatComparisons(comparisons)
	documentationText := formatting.FormatDocumentations(documentation)

	userPrompt, err := user.RenderUserPrompt(diffContent, documentationText, userGuidance, truncation.TruncationMetadata{})
	if err != nil {
		return 0, "", fmt.Errorf("failed to format user prompt: %w", err)
	}

	// Try LLM analysis with full content first
	response, err := ra.llmClient.Analyze(userPrompt)
	var truncationInfo *truncation.TruncationMetadata

	if err != nil {
		// Check if this is a context window error
		contextErr, ok := err.(*llmerrors.ContextWindowError)
		if !ok {
			return 0, "", fmt.Errorf("failed to analyze: %w", err)
		}

		// Retry with progressive truncation
		slog.Warn("Context window exceeded, retrying with progressive truncation",
			"provider", contextErr.Provider,
			"status_code", contextErr.StatusCode)

		response, truncationInfo, err = ra.retryWithTruncation(comparisons, documentation, userGuidance)
		if err != nil {
			return 0, "", err
		}
	}

	// Generate report
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

	score, finalReport, err := report.GenerateReport(reportConfig)
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate report: %w", err)
	}

	return float64(score), finalReport, nil
}

// retryWithTruncation attempts LLM analysis with progressively more aggressive truncation
func (ra *ReleaseAnalyzer) retryWithTruncation(comparisons []*types.Comparison, documentation []*types.Documentation, userGuidance []types.UserGuidance) (string, *truncation.TruncationMetadata, error) {
	var lastErr error
	for _, level := range truncationLevels {
		slog.Info("Attempting analysis with truncation", "level", level)

		truncatedComparisons, metadata := truncation.TruncateMultipleComparisons(comparisons, level)
		truncatedDocs := truncation.TruncateDocumentation(documentation, level)

		userPrompt, err := user.RenderUserPrompt(
			formatting.FormatComparisons(truncatedComparisons),
			formatting.FormatDocumentations(truncatedDocs),
			userGuidance,
			metadata,
		)
		if err != nil {
			return "", nil, fmt.Errorf("failed to format user prompt with %s truncation: %w", level, err)
		}

		response, err := ra.llmClient.Analyze(userPrompt)
		if err == nil {
			slog.Info("Analysis succeeded with truncation", "level", level)
			return response, &metadata, nil
		}

		if _, isContextErr := err.(*llmerrors.ContextWindowError); isContextErr {
			slog.Warn("Context window still exceeded with truncation", "level", level)
			lastErr = err
			continue
		}

		return "", nil, fmt.Errorf("failed to analyze with %s truncation: %w", level, err)
	}

	return "", nil, fmt.Errorf("failed to analyze even with extreme truncation: %w", lastErr)
}
