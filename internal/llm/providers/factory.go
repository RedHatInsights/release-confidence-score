package providers

import (
	"fmt"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/llm"
)

// NewClient creates the appropriate LLM client based on configuration
func NewClient() (llm.LLMClient, error) {
	cfg := config.Get()

	switch cfg.ModelProvider {
	case "claude":
		return NewClaude(), nil

	case "gemini":
		return NewGemini(), nil

	case "llama":
		return NewLlama(), nil

	default:
		return nil, fmt.Errorf("unsupported model provider: %s", cfg.ModelProvider)
	}
}
