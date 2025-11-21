package system

import (
	_ "embed"
	"log/slog"

	"release-confidence-score/internal/config"
)

//go:embed system_prompt_v1.md
var systemPromptV1 string

//go:embed system_prompt_v2.md
var systemPromptV2 string

// GetSystemPrompt returns the appropriate system prompt based on the config's SystemPromptVersion
func GetSystemPrompt(cfg *config.Config) string {
	version := cfg.SystemPromptVersion

	switch version {
	case "v1":
		slog.Debug("Using system prompt v1")
		return systemPromptV1
	case "v2":
		slog.Debug("Using system prompt v2")
		return systemPromptV2
	default:
		slog.Warn("Unknown system prompt version, falling back to v1",
			"version", version,
			"supported_versions", []string{"v1", "v2"})
		return systemPromptV1
	}
}
