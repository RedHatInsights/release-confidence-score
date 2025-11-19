package llm

import (
	"fmt"
	"log/slog"

	"release-confidence-score/internal/git/github"
	"release-confidence-score/internal/llm/prompts/user"
	"release-confidence-score/internal/qe"
	"release-confidence-score/internal/shared"
)

// AnalyzeWithProgressiveTruncation performs LLM analysis with automatic retry using progressive truncation
// Returns the LLM response, truncation metadata (nil if no truncation), and any error
func AnalyzeWithProgressiveTruncation(
	client LLMClient,
	comparisons []*github.CompareData,
	documentation []*github.RepoDocumentation,
	allUserGuidance []shared.UserGuidance,
	qeTestingCommits *qe.TestingCommits,
) (string, *shared.TruncationMetadata, error) {
	// Phase 2: Format data and prepare prompts
	// Format diff content (no truncation)
	diffContent := github.FormatMultipleComparisons(comparisons)

	// Format documentation for LLM analysis
	documentationText := github.FormatMultipleRepoDocumentationForLLM(documentation)

	// Render initial user prompt with full diff
	userPrompt, err := user.RenderUserPrompt(diffContent, documentationText, allUserGuidance, qeTestingCommits, shared.TruncationMetadata{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to format initial user prompt: %w", err)
	}

	// Phase 3: Submit prompt
	// Try with full diff first
	response, err := client.Analyze(userPrompt)
	if err == nil {
		return response, nil, nil
	}

	// Check if this is a context window error
	contextErr, ok := err.(*ContextWindowError)
	if !ok {
		return "", nil, fmt.Errorf("failed to analyze merge request: %w", err)
	}

	// Phase 4: Reformat with truncation and resubmit
	// Context window exceeded - try with progressive truncation
	slog.Warn("Context window exceeded, retrying with progressive truncation",
		"provider", contextErr.Provider,
		"status_code", contextErr.StatusCode)

	truncationLevels := []github.TruncationLevel{
		github.TruncationModerate,
		github.TruncationAggressive,
		github.TruncationExtreme,
		github.TruncationUltimate,
	}

	for _, level := range truncationLevels {
		levelName := map[github.TruncationLevel]string{
			github.TruncationModerate:   "moderate",
			github.TruncationAggressive: "aggressive",
			github.TruncationExtreme:    "extreme",
			github.TruncationUltimate:   "ultimate",
		}[level]

		slog.Info("Attempting analysis with truncation", "level", levelName)

		// Truncate diffs and reformat prompt
		truncatedDiff, metadata := github.TruncateMultipleComparisonsWithStaticPatterns(comparisons, level)
		userPrompt, err := user.RenderUserPrompt(truncatedDiff, documentationText, allUserGuidance, qeTestingCommits, metadata)
		if err != nil {
			return "", nil, fmt.Errorf("failed to format user prompt with %s truncation: %w", levelName, err)
		}

		// Retry analysis with truncated prompt
		response, err = client.Analyze(userPrompt)
		if err != nil {
			// Check if still a context window error
			if _, isContextErr := err.(*ContextWindowError); isContextErr {
				slog.Warn("Context window still exceeded with truncation", "level", levelName)
				continue
			}
			// Different error - fail immediately
			return "", nil, fmt.Errorf("failed to analyze merge request with %s truncation: %w", levelName, err)
		}

		// Success!
		slog.Info("Analysis succeeded with truncation", "level", levelName)
		return response, &metadata, nil
	}

	// Exhausted all truncation levels
	return "", nil, fmt.Errorf("failed to analyze merge request even with extreme truncation: %w", err)
}
