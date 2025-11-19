package user

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"release-confidence-score/internal/qe"
	"release-confidence-score/internal/shared"
)

//go:embed user_prompt_template_v1.md
var userPromptTemplateV1 string

// PromptData holds the data for the user prompt template
type PromptData struct {
	Diff               string
	Documentation      string
	QETesting          *qe.TestingCommits         // QE testing commits grouped by status and repository
	TruncationMetadata *shared.TruncationMetadata // Optional truncation information
	UserGuidance       []string
}

// RenderUserPrompt formats the user prompt with actual diff, conditionally including documentation, user guidance, detailed QE testing label information, and truncation metadata
func RenderUserPrompt(diff, documentation string, userGuidance []shared.UserGuidance, qeTesting *qe.TestingCommits, truncationMetadata shared.TruncationMetadata) (string, error) {

	// Extract authorized guidance content for LLM (security: only use vetted guidance)
	var authorizedGuidanceContent []string
	for _, g := range userGuidance {
		if g.IsAuthorized {
			authorizedGuidanceContent = append(authorizedGuidanceContent, g.Content)
		}
	}

	// Create template data
	data := PromptData{
		Diff:               diff,
		Documentation:      documentation,
		QETesting:          qeTesting,
		TruncationMetadata: nil, // Default to no truncation
		UserGuidance:       authorizedGuidanceContent,
	}

	// Only include truncation metadata if it's meaningful
	if truncationMetadata.Truncated {
		data.TruncationMetadata = &truncationMetadata
	}

	// Parse template file
	tmpl, err := template.New("user_prompt").Parse(userPromptTemplateV1)
	if err != nil {
		return "", fmt.Errorf("failed to parse user prompt template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute user prompt template: %w", err)
	}

	return buf.String(), nil
}
