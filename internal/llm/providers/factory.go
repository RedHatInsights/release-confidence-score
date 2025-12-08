package providers

import (
	"fmt"

	"release-confidence-score/internal/config"
)

// NewClient creates the appropriate LLM client based on configuration
func NewClient(cfg *config.Config) (LLMClient, error) {
	switch cfg.ModelProvider {
	case "claude":
		return NewClaude(cfg), nil

	case "gemini":
		return NewGemini(cfg), nil

	case "llama":
		return NewLlama(cfg), nil

	default:
		return nil, fmt.Errorf("unsupported model provider: %s", cfg.ModelProvider)
	}
}
