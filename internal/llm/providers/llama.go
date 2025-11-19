package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/llm"
	"release-confidence-score/internal/llm/prompts/system"
	"release-confidence-score/internal/logger"
	"release-confidence-score/internal/shared"
)

type LlamaClient struct{}

type LlamaRequest struct {
	MaxTokens   int     `json:"max_tokens"`
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
}

type LlamaResponse struct {
	Choices []LlamaChoice `json:"choices"`
	Usage   LlamaUsage    `json:"usage"`
}

type LlamaChoice struct {
	Text string `json:"text"`
}

type LlamaUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func NewLlama() llm.LLMClient {
	return &LlamaClient{}
}

func (l *LlamaClient) Analyze(userPrompt string) (string, error) {
	cfg := config.Get()

	// Llama uses combined prompt
	combinedPrompt := system.GetSystemPrompt() + "\n\n" + userPrompt

	req := LlamaRequest{
		Model:       cfg.ModelID,
		Prompt:      combinedPrompt,
		MaxTokens:   cfg.MaxResponseTokens,
		Temperature: 0,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	slog.Log(context.Background(), logger.LevelTrace, "Llama API request", "request", jsonData)

	url := cfg.ModelAPI + "/v1/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.UserKey)

	httpClient := shared.NewHTTPClient(shared.HTTPClientOptions{
		Timeout:       time.Duration(cfg.TimeoutSeconds) * time.Second,
		SkipSSLVerify: cfg.ModelSkipSSLVerify,
	})

	slog.Debug("Sending release analysis request to LLM", "provider", "Llama", "model", cfg.ModelID)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Check if this is a context window error
		if llm.IsContextWindowError(resp.StatusCode, body) {
			return "", &llm.ContextWindowError{
				StatusCode: resp.StatusCode,
				Message:    string(body),
				Provider:   "Llama",
			}
		}
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response LlamaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	slog.Log(context.Background(), logger.LevelTrace, "Llama API response", "response", response)

	slog.Debug("Llama API token usage",
		"input_tokens", response.Usage.PromptTokens,
		"output_tokens", response.Usage.CompletionTokens,
		"total_tokens", response.Usage.TotalTokens)

	return response.Choices[0].Text, nil
}
