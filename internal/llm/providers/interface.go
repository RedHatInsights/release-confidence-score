package providers

// LLMClient interface for all LLM providers
type LLMClient interface {
	Analyze(userPrompt string) (string, error)
}
