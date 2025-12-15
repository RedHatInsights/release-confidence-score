package truncation

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"release-confidence-score/internal/git/types"
)

// Truncation level constants
const (
	LevelLow      = "low"
	LevelModerate = "moderate"
	LevelHigh     = "high"
	LevelExtreme  = "extreme"
)

// Small file threshold constants (in lines)
// Files smaller than these thresholds are never truncated
const (
	SmallFileThresholdLow      = 100
	SmallFileThresholdModerate = 75
	SmallFileThresholdHigh     = 50
	SmallFileThresholdExtreme  = 20
)

// FileRiskLevel represents the risk level of a file for prioritization during truncation
type FileRiskLevel int

const (
	RiskCritical FileRiskLevel = iota
	RiskHigh
	RiskMedium
	RiskLow
)

// TruncationMetadata contains information about diff truncation applied during LLM analysis
type TruncationMetadata struct {
	Truncated          bool     // Whether truncation was applied
	Level              string   // Truncation level: "moderate", "aggressive", "extreme", "ultimate"
	FilesPreserved     int      // Number of files kept in full
	FilesTruncated     int      // Number of files that were truncated
	TruncatedFilesList []string // List of truncated file paths
	TotalFiles         int      // Total number of files in the diff
}

// truncationConfig holds the parameters for a specific truncation level
type truncationConfig struct {
	keepStart int
	keepEnd   int
}

// Truncation level configurations
var truncationLevels = map[string]truncationConfig{
	LevelLow:      {keepStart: 50, keepEnd: 20},
	LevelModerate: {keepStart: 20, keepEnd: 10},
	LevelHigh:     {keepStart: 10, keepEnd: 5},
	LevelExtreme:  {keepStart: 5, keepEnd: 3},
}

// Embedded risk patterns JSON file
//
//go:embed risk_patterns.json
var riskPatternsJSON []byte

// File risk classification patterns - loaded from JSON file at initialization
var riskPatterns map[FileRiskLevel][]string

// init loads the risk patterns from the embedded JSON file
func init() {
	// Parse the JSON structure
	var patterns struct {
		Critical []string `json:"critical"`
		High     []string `json:"high"`
		Medium   []string `json:"medium"`
		Low      []string `json:"low"`
	}

	if err := json.Unmarshal(riskPatternsJSON, &patterns); err != nil {
		// Panic on JSON parse error since the file is embedded at compile time
		// This indicates a programming error that should be caught during development
		panic(fmt.Sprintf("Failed to parse embedded risk_patterns.json: %v", err))
	}

	// Map JSON fields to FileRiskLevel enum
	riskPatterns = map[FileRiskLevel][]string{
		RiskCritical: patterns.Critical,
		RiskHigh:     patterns.High,
		RiskMedium:   patterns.Medium,
		RiskLow:      patterns.Low,
	}
}

// TruncateMultipleComparisons truncates multiple comparisons and returns combined metadata
// This is the main entry point for the truncation system
func TruncateMultipleComparisons(comparisons []*types.Comparison, level string) ([]*types.Comparison, TruncationMetadata) {
	// Normalize level string once at entry point
	normalizedLevel := strings.ToLower(level)

	truncated := make([]*types.Comparison, len(comparisons))
	metadataList := make([]*TruncationMetadata, len(comparisons))

	for i, comparison := range comparisons {
		truncated[i], metadataList[i] = truncateComparison(comparison, normalizedLevel)
	}

	combinedMetadata := combineMetadata(metadataList, normalizedLevel)
	return truncated, combinedMetadata
}

// truncateComparison truncates a single comparison using risk-based selective truncation
// Returns a new truncated comparison and metadata about what was truncated
func truncateComparison(comparison *types.Comparison, level string) (*types.Comparison, *TruncationMetadata) {
	if comparison == nil {
		return nil, nil
	}

	keepStart, keepEnd := getTruncationParams(level)

	// Create a copy of the comparison (Files are copied deeply since we modify Patch)
	truncated := &types.Comparison{
		RepoURL: comparison.RepoURL,
		Commits: comparison.Commits,
		Files:   make([]types.FileChange, len(comparison.Files)),
		Stats:   comparison.Stats,
	}
	copy(truncated.Files, comparison.Files)

	// Initialize truncation metadata
	metadata := &TruncationMetadata{
		Truncated:          false,
		Level:              level,
		TotalFiles:         len(truncated.Files),
		FilesPreserved:     0,
		FilesTruncated:     0,
		TruncatedFilesList: []string{},
	}

	smallFileThreshold := getSmallFileThreshold(level)

	// Process each file
	for i := range truncated.Files {
		file := &truncated.Files[i]

		// Skip files with no patch
		if file.Patch == "" {
			metadata.FilesPreserved++
			continue
		}

		// Skip truncation for small files
		if countLines(file.Patch) < smallFileThreshold {
			metadata.FilesPreserved++
			continue
		}

		// Determine if this file should be truncated based on risk level
		fileRisk := classifyFileRisk(file.Filename)
		if !shouldTruncateFile(fileRisk, level) {
			// Preserve this file completely
			metadata.FilesPreserved++
			continue
		}

		// Truncate the patch
		originalPatch := file.Patch
		truncatedPatch := truncatePatch(originalPatch, keepStart, keepEnd)

		if truncatedPatch != originalPatch {
			file.Patch = truncatedPatch
			metadata.Truncated = true
			metadata.FilesTruncated++
			metadata.TruncatedFilesList = append(metadata.TruncatedFilesList, file.Filename)
		} else {
			metadata.FilesPreserved++
		}
	}

	slog.Debug("Truncated comparison",
		"level", level,
		"total_files", metadata.TotalFiles,
		"preserved", metadata.FilesPreserved,
		"truncated", metadata.FilesTruncated)

	return truncated, metadata
}

// getTruncationParams returns the truncation parameters for the given level
// Falls back to low level if the level is unknown
// Expects level to already be normalized (lowercased)
func getTruncationParams(level string) (keepStart int, keepEnd int) {
	config, exists := truncationLevels[level]
	if !exists {
		// Fall back to low level
		config = truncationLevels[LevelLow]
	}
	return config.keepStart, config.keepEnd
}

// getSmallFileThreshold returns the threshold (in lines) below which files are never truncated
// The threshold decreases as truncation becomes more aggressive
// Expects level to already be normalized (lowercased)
func getSmallFileThreshold(level string) int {
	switch level {
	case LevelLow:
		return SmallFileThresholdLow
	case LevelModerate:
		return SmallFileThresholdModerate
	case LevelHigh:
		return SmallFileThresholdHigh
	case LevelExtreme:
		return SmallFileThresholdExtreme
	default:
		return SmallFileThresholdLow
	}
}

// countLines returns the number of lines in the given text
func countLines(text string) int {
	if text == "" {
		return 0
	}
	// Count newlines and add 1 (a string with N newlines has N+1 lines)
	return strings.Count(text, "\n") + 1
}

// classifyFileRisk determines the risk level of a file based on its filename
func classifyFileRisk(filename string) FileRiskLevel {
	lower := strings.ToLower(filename)

	// Check patterns in order of risk level (highest to lowest)
	for _, risk := range []FileRiskLevel{RiskCritical, RiskHigh, RiskMedium, RiskLow} {
		if matchesAnyPattern(lower, riskPatterns[risk]) {
			return risk
		}
	}

	// Default to medium risk if no pattern matches
	return RiskMedium
}

// matchesAnyPattern checks if the filename matches any of the given glob patterns
func matchesAnyPattern(filename string, patterns []string) bool {
	// Split filename once before looping through patterns for efficiency
	parts := strings.Split(filename, "/")

	for _, pattern := range patterns {
		// Try glob pattern matching on the full filename first
		matched, err := filepath.Match(pattern, filename)
		if err != nil {
			slog.Warn("Invalid glob pattern", "pattern", pattern, "error", err)
			continue
		}
		if matched {
			return true
		}

		// Fallback: check if pattern matches any path component
		// This handles patterns like "auth*", "*/tests/*", etc.
		for _, part := range parts {
			matched, err := filepath.Match(pattern, part)
			if err != nil {
				// Already warned about this pattern above
				break
			}
			if matched {
				return true
			}
		}
	}
	return false
}

// shouldTruncateFile determines if a file should be truncated based on its risk level
// and the current truncation level
// Expects level to already be normalized (lowercased)
func shouldTruncateFile(risk FileRiskLevel, level string) bool {
	switch risk {
	case RiskLow:
		// Always truncate low-risk files (docs, tests, generated files)
		return true
	case RiskMedium:
		// Truncate medium-risk files in moderate, high, and extreme modes
		return level == LevelModerate ||
			level == LevelHigh ||
			level == LevelExtreme
	case RiskHigh:
		// Truncate high-risk files in high and extreme modes
		return level == LevelHigh ||
			level == LevelExtreme
	case RiskCritical:
		// NEVER truncate critical risk files (auth, DB, API definitions)
		return false
	}
	return false
}

// truncatePatch truncates a patch to keep only the first keepStart and last keepEnd lines
// Returns the original patch if it's shorter than keepStart + keepEnd lines
func truncatePatch(patch string, keepStart, keepEnd int) string {
	if patch == "" {
		return patch
	}

	lines := strings.Split(patch, "\n")
	totalLines := len(lines)

	// Don't truncate if the patch is already small enough
	if totalLines <= keepStart+keepEnd {
		return patch
	}

	// Preserve whether original patch ended with newline
	endsWithNewline := strings.HasSuffix(patch, "\n")

	var result strings.Builder

	// Write the first keepStart lines
	for i := 0; i < keepStart && i < totalLines; i++ {
		result.WriteString(lines[i])
		result.WriteString("\n")
	}

	// Write the omission marker
	omittedLines := totalLines - keepStart - keepEnd
	result.WriteString(fmt.Sprintf("\n... [%d lines omitted] ...\n\n", omittedLines))

	// Write the last keepEnd lines
	startEnd := totalLines - keepEnd
	for i := startEnd; i < totalLines; i++ {
		result.WriteString(lines[i])
		// Add newline for all lines except the last one (unless original ended with newline)
		if i < totalLines-1 || endsWithNewline {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// combineMetadata combines multiple truncation metadata objects into one
func combineMetadata(metadataList []*TruncationMetadata, level string) TruncationMetadata {
	combined := TruncationMetadata{
		Level:              level,
		Truncated:          false,
		TotalFiles:         0,
		FilesPreserved:     0,
		FilesTruncated:     0,
		TruncatedFilesList: []string{},
	}

	for _, metadata := range metadataList {
		if metadata == nil {
			continue
		}
		if metadata.Truncated {
			combined.Truncated = true
		}
		combined.TotalFiles += metadata.TotalFiles
		combined.FilesPreserved += metadata.FilesPreserved
		combined.FilesTruncated += metadata.FilesTruncated
		combined.TruncatedFilesList = append(combined.TruncatedFilesList, metadata.TruncatedFilesList...)
	}

	return combined
}

// TruncateDocumentation removes linked docs for high/extreme truncation levels
// This reduces context size by keeping only the entry point documentation
func TruncateDocumentation(docs []*types.Documentation, level string) []*types.Documentation {
	// Only truncate linked docs at high and extreme levels
	if level != LevelHigh && level != LevelExtreme {
		return docs
	}

	result := make([]*types.Documentation, len(docs))
	for i, doc := range docs {
		if doc == nil {
			result[i] = nil
			continue
		}

		// Create a copy with linked docs removed
		truncated := &types.Documentation{
			Repository:            doc.Repository,
			MainDocContent:        doc.MainDocContent,
			MainDocFile:           doc.MainDocFile,
			AdditionalDocsContent: make(map[string]string),
			AdditionalDocsOrder:   []string{},
		}

		result[i] = truncated

		if len(doc.AdditionalDocsContent) > 0 {
			slog.Debug("Truncated documentation",
				"level", level,
				"repo", doc.Repository.URL,
				"removed_additional_docs", len(doc.AdditionalDocsContent))
		}
	}

	return result
}
