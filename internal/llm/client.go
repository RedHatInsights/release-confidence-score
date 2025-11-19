package llm

// LLMClient interface for all LLM providers
type LLMClient interface {
	Analyze(userPrompt string) (string, error)
}
