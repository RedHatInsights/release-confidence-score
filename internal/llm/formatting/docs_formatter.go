package formatting

import (
	"strings"

	"release-confidence-score/internal/git/types"
)

// headingIncrement is the number of levels to increment documentation headings
// to nest under "### filename (repo)" (H3) headers in the prompt.
// This makes # → ####, ## → #####, etc., maintaining proper markdown hierarchy.
const headingIncrement = 3

// FormatDocumentations formats multiple documentation sets for LLM consumption
func FormatDocumentations(docsList []*types.Documentation) string {
	if len(docsList) == 0 {
		return ""
	}

	var result strings.Builder

	for _, docs := range docsList {
		if docs == nil || docs.MainDocContent == "" {
			continue
		}

		repoName := docs.Repository.Owner + "/" + docs.Repository.Name

		result.WriteString("### " + docs.MainDocFile + " (" + repoName + ")\n\n")
		result.WriteString(adjustMarkdownHeadingLevels(docs.MainDocContent, headingIncrement))
		result.WriteString("\n\n")

		for _, displayName := range docs.AdditionalDocsOrder {
			if content, exists := docs.AdditionalDocsContent[displayName]; exists {
				result.WriteString("### " + displayName + " (" + repoName + ")\n\n")
				result.WriteString(adjustMarkdownHeadingLevels(content, headingIncrement))
				result.WriteString("\n\n")
			}
		}
	}

	return result.String()
}

// adjustMarkdownHeadingLevels increments all markdown heading levels by the specified amount
// to maintain proper hierarchy when embedding documentation into prompts.
// Headings are capped at H6 (######). Headings inside code blocks are not modified.
func adjustMarkdownHeadingLevels(content string, incrementBy int) string {
	if incrementBy <= 0 || content == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	inCodeBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track code block boundaries
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock || !strings.HasPrefix(line, "#") {
			continue
		}

		// Count leading # symbols
		hashCount := 0
		for _, ch := range line {
			if ch == '#' {
				hashCount++
			} else {
				break
			}
		}

		// Add prefix but cap at H6
		newLevel := hashCount + incrementBy
		if newLevel > 6 {
			newLevel = 6
		}
		lines[i] = strings.Repeat("#", newLevel) + line[hashCount:]
	}

	return strings.Join(lines, "\n")
}
