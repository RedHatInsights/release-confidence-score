package user

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"release-confidence-score/internal/git/shared"
	"release-confidence-score/internal/git/types"
	"release-confidence-score/internal/llm/truncation"
)

//go:embed user_prompt_template_v1.md
var userPromptTemplateV1 string

var userPromptTemplate *template.Template

func init() {
	userPromptTemplate = template.Must(
		template.New("user_prompt").Parse(userPromptTemplateV1),
	)
}

// PromptData holds the data for the user prompt template
type PromptData struct {
	Diff               string
	Documentation      string
	QETesting          *shared.TestingCommits         // QE testing commits grouped by status and repository
	TruncationMetadata *truncation.TruncationMetadata // Optional truncation information
	UserGuidance       []string
}

// RenderUserPrompt formats the user prompt with actual diff, conditionally including documentation, user guidance, detailed QE testing label information, and truncation metadata
func RenderUserPrompt(diff, documentation string, userGuidance []types.UserGuidance, qeTesting *shared.TestingCommits, truncationMetadata truncation.TruncationMetadata) (string, error) {
	// Create template data
	data := PromptData{
		Diff:          diff,
		Documentation: documentation,
		QETesting:     qeTesting,
		UserGuidance:  extractAuthorizedGuidance(userGuidance),
	}

	// Only include truncation metadata if it's meaningful
	if truncationMetadata.Truncated {
		data.TruncationMetadata = &truncationMetadata
	}

	// Execute pre-compiled template
	var buf bytes.Buffer
	if err := userPromptTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute user prompt template: %w", err)
	}

	return buf.String(), nil
}

// extractAuthorizedGuidance filters user guidance to only include authorized content
func extractAuthorizedGuidance(userGuidance []types.UserGuidance) (authorized []string) {
	for _, g := range userGuidance {
		if g.IsAuthorized {
			authorized = append(authorized, g.Content)
		}
	}
	return
}
