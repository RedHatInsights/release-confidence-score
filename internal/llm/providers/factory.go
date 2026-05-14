package providers

import (
	"fmt"

	"golang.org/x/oauth2"

	"release-confidence-score/internal/config"
)

func NewClient(cfg *config.Config, ts oauth2.TokenSource) (LLMClient, error) {
	switch cfg.ModelProvider {
	case "claude":
		return NewClaude(cfg, ts), nil

	case "gemini":
		return NewGemini(cfg, ts), nil

	default:
		return nil, fmt.Errorf("unsupported model provider: %s", cfg.ModelProvider)
	}
}
