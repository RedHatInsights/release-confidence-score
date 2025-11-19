package github

import (
	"fmt"
	"strings"

	"release-confidence-score/internal/shared"

	"github.com/google/go-github/v76/github"
)

// FileRiskLevel represents the risk level of a file for prioritization during truncation
type FileRiskLevel int

const (
	RiskCritical FileRiskLevel = iota // Preserve these files first
	RiskHigh
	RiskMedium
	RiskLow // Truncate these files first
)

// TruncationLevel represents how aggressively to truncate patches
type TruncationLevel int

const (
	TruncationModerate   TruncationLevel = iota // Keep first 50 + last 20 lines
	TruncationAggressive                        // Keep first 20 + last 10 lines
	TruncationExtreme                           // Keep first 10 + last 5 lines
	TruncationUltimate                          // Keep first 5 + last 3 lines
)

// ClassifyFileRisk determines the risk level of a file based on language-agnostic patterns
func ClassifyFileRisk(filename string) FileRiskLevel {
	lower := strings.ToLower(filename)

	// Critical Risk: Database, auth, security, API contracts
	criticalPatterns := []string{
		".sql", "migration", "schema", "alembic/", "flyway/",
		"auth", "security", "token", "credential", "permission", "oauth",
		"api/", "openapi.", "swagger.", ".proto", ".graphql",
	}
	for _, pattern := range criticalPatterns {
		if strings.Contains(lower, pattern) {
			return RiskCritical
		}
	}

	// High Risk: Infrastructure, deployment, config, build
	highPatterns := []string{
		"dockerfile", "docker-compose", ".tf", ".tfvars",
		"deploy/", "kubernetes/", "k8s/", "helm/", ".github/workflows/",
		".gitlab-ci.yml", "jenkinsfile",
		"config/", ".config.", ".env.example", ".properties", ".ini", ".toml",
		"makefile", "cmakelists.txt", "build.gradle", "pom.xml", ".csproj",
	}
	for _, pattern := range highPatterns {
		if strings.Contains(lower, pattern) {
			return RiskHigh
		}
	}

	// Medium Risk: Dependencies, lock files, data processing
	mediumPatterns := []string{
		"package.json", "go.mod", "requirements.txt", "gemfile", "cargo.toml",
		"composer.json", "-lock.", ".lock", "go.sum", "gemfile.lock", "cargo.lock",
		"pipeline", "etl", "batch",
	}
	for _, pattern := range mediumPatterns {
		if strings.Contains(lower, pattern) {
			return RiskMedium
		}
	}

	// Low Risk: Documentation, tests, linting, IDE config, generated files
	lowPatterns := []string{
		// Documentation
		".md", ".txt", ".rst", ".adoc", "docs/", "changelog", "readme",
		// Tests (all languages)
		"_test.go", "_test_", "test_", "_test.py", "tests/",
		".test.js", ".spec.js", ".test.ts", ".spec.ts", "__tests__/",
		"test.java", "tests.java", "src/test/",
		"_spec.rb", "spec/",
		".tests.cs", ".test.cs",
		// Linting/formatting
		".eslintrc", ".prettierrc", ".editorconfig", ".pylintrc", "rustfmt.toml",
		// IDE config
		".vscode/", ".idea/", ".iml",
		// Generated files
		"generated", ".min.js", ".bundle.js", "vendor/", "node_modules/",
	}
	for _, pattern := range lowPatterns {
		if strings.Contains(lower, pattern) {
			return RiskLow
		}
	}

	// Default to medium risk if no pattern matches
	return RiskMedium
}

// TruncatePatch truncates a unified diff patch while preserving context
func TruncatePatch(patch string, level TruncationLevel) string {
	if patch == "" {
		return patch
	}

	lines := strings.Split(patch, "\n")
	totalLines := len(lines)

	// Determine how many lines to keep
	var headLines, tailLines int
	switch level {
	case TruncationModerate:
		headLines, tailLines = 50, 20
	case TruncationAggressive:
		headLines, tailLines = 20, 10
	case TruncationExtreme:
		headLines, tailLines = 10, 5
	case TruncationUltimate:
		headLines, tailLines = 5, 3
	}

	// Don't truncate if the patch is small enough
	if totalLines <= headLines+tailLines {
		return patch
	}

	// Keep the beginning
	result := strings.Join(lines[:headLines], "\n")

	// Add truncation marker
	removedLines := totalLines - headLines - tailLines
	result += fmt.Sprintf("\n... [TRUNCATED: ~%d lines removed for brevity] ...\n", removedLines)

	// Keep the end
	result += strings.Join(lines[len(lines)-tailLines:], "\n")

	return result
}

// TruncateDiffByRisk truncates a diff comparison based on file risk levels
func TruncateDiffByRisk(comparison *github.CommitsComparison, level TruncationLevel) (*github.CommitsComparison, shared.TruncationMetadata) {
	if comparison == nil || comparison.Files == nil {
		return comparison, shared.TruncationMetadata{}
	}

	// Create a copy to avoid modifying the original
	truncated := &github.CommitsComparison{
		BaseCommit:      comparison.BaseCommit,
		MergeBaseCommit: comparison.MergeBaseCommit,
		Status:          comparison.Status,
		AheadBy:         comparison.AheadBy,
		BehindBy:        comparison.BehindBy,
		TotalCommits:    comparison.TotalCommits,
		Commits:         comparison.Commits,
		Files:           make([]*github.CommitFile, len(comparison.Files)),
	}

	// Classify files by risk level
	type fileWithRisk struct {
		file *github.CommitFile
		risk FileRiskLevel
	}

	filesWithRisk := make([]fileWithRisk, len(comparison.Files))
	for i, file := range comparison.Files {
		filename := ""
		if file.Filename != nil {
			filename = *file.Filename
		}
		filesWithRisk[i] = fileWithRisk{
			file: file,
			risk: ClassifyFileRisk(filename),
		}
	}

	// Track truncation metadata
	metadata := shared.TruncationMetadata{
		Truncated:          false, // Will be set to true if any file is truncated
		TotalFiles:         len(comparison.Files),
		FilesPreserved:     0,
		FilesTruncated:     0,
		TruncatedFilesList: []string{},
	}

	switch level {
	case TruncationModerate:
		metadata.Level = "moderate"
	case TruncationAggressive:
		metadata.Level = "aggressive"
	case TruncationExtreme:
		metadata.Level = "extreme"
	case TruncationUltimate:
		metadata.Level = "ultimate"
	}

	// Process files: truncate low-risk files first
	for i, fwr := range filesWithRisk {
		file := fwr.file
		truncated.Files[i] = &github.CommitFile{
			SHA:              file.SHA,
			Filename:         file.Filename,
			Additions:        file.Additions,
			Deletions:        file.Deletions,
			Changes:          file.Changes,
			Status:           file.Status,
			PreviousFilename: file.PreviousFilename,
			BlobURL:          file.BlobURL,
			RawURL:           file.RawURL,
			ContentsURL:      file.ContentsURL,
		}

		// Skip truncation for small files (threshold depends on truncation level)
		if file.Patch != nil {
			patchLines := len(strings.Split(*file.Patch, "\n"))
			smallFileThreshold := 100 // Default threshold
			if level == TruncationUltimate {
				smallFileThreshold = 20 // Much lower threshold in ultimate mode
			}
			if patchLines < smallFileThreshold {
				// Keep the full patch
				truncated.Files[i].Patch = file.Patch
				metadata.FilesPreserved++
				continue
			}
		}

		// Decide whether to truncate based on risk level
		shouldTruncate := false
		switch fwr.risk {
		case RiskLow:
			// Always truncate low-risk files
			shouldTruncate = true
		case RiskMedium:
			// Truncate medium-risk files in aggressive, extreme, and ultimate modes
			if level == TruncationAggressive || level == TruncationExtreme || level == TruncationUltimate {
				shouldTruncate = true
			}
		case RiskHigh:
			// Truncate high-risk files in extreme and ultimate modes
			if level == TruncationExtreme || level == TruncationUltimate {
				shouldTruncate = true
			}
		case RiskCritical:
			// Truncate critical risk files only in ultimate mode
			if level == TruncationUltimate {
				shouldTruncate = true
			}
		}

		if shouldTruncate && file.Patch != nil {
			// Truncate the patch
			truncatedPatch := TruncatePatch(*file.Patch, level)
			truncated.Files[i].Patch = &truncatedPatch
			metadata.Truncated = true
			metadata.FilesTruncated++
			if file.Filename != nil {
				metadata.TruncatedFilesList = append(metadata.TruncatedFilesList, *file.Filename)
			}
		} else {
			// Keep the full patch
			truncated.Files[i].Patch = file.Patch
			metadata.FilesPreserved++
		}
	}

	return truncated, metadata
}

// TruncateMultipleComparisonsWithStaticPatterns uses static pattern matching to classify file risk,
// then applies specified truncation level across multiple comparisons.
// Returns formatted diff string and combined metadata across all comparisons.
func TruncateMultipleComparisonsWithStaticPatterns(comparisons []*CompareData, level TruncationLevel) (string, shared.TruncationMetadata) {
	var allDiffs strings.Builder
	var combinedMetadata shared.TruncationMetadata

	for i, compareData := range comparisons {
		if compareData == nil || compareData.Comparison == nil {
			continue
		}

		allDiffs.WriteString(fmt.Sprintf("=== Diff %d: %s ===\n", i+1, compareData.CompareURL))

		// Use static pattern matching to classify file risk, then apply specified truncation level
		truncatedComparison, metadata := TruncateDiffByRisk(compareData.Comparison, level)

		// Format the truncated comparison
		formattedDiff := FormatComparisonForLLM(truncatedComparison, compareData.AllCommits, compareData.CompareURL)
		allDiffs.WriteString(formattedDiff)
		allDiffs.WriteString("\n\n")

		// Aggregate metadata across all comparisons
		if metadata.Truncated {
			combinedMetadata.Truncated = true
		}
		combinedMetadata.TotalFiles += metadata.TotalFiles
		combinedMetadata.FilesPreserved += metadata.FilesPreserved
		combinedMetadata.FilesTruncated += metadata.FilesTruncated
		combinedMetadata.TruncatedFilesList = append(combinedMetadata.TruncatedFilesList, metadata.TruncatedFilesList...)
		combinedMetadata.Level = metadata.Level
	}

	return allDiffs.String(), combinedMetadata
}
