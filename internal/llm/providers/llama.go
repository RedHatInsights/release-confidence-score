package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"release-confidence-score/internal/config"
	httputil "release-confidence-score/internal/http"
	llmerrors "release-confidence-score/internal/llm/errors"
	"release-confidence-score/internal/llm/prompts/system"
)

type LlamaClient struct {
	config *config.Config
}

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

func NewLlama(cfg *config.Config) LLMClient {
	return &LlamaClient{config: cfg}
}

func (l *LlamaClient) Analyze(userPrompt string) (string, error) {
	cfg := l.config

	// Llama uses combined prompt
	combinedPrompt := system.GetSystemPrompt(cfg) + "\n\n" + userPrompt

	req := LlamaRequest{
		Model:       cfg.ModelID,
		Prompt:      combinedPrompt,
		MaxTokens:   cfg.ModelMaxResponseTokens,
		Temperature: 0,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	slog.Debug("Llama API request", "request", jsonData)

	url := cfg.ModelAPI + "/v1/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.ModelUserKey)

	httpClient := httputil.NewHTTPClient(httputil.HTTPClientOptions{
		Timeout:       time.Duration(cfg.ModelTimeoutSeconds) * time.Second,
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
		if llmerrors.IsContextWindowError(resp.StatusCode, body) {
			return "", &llmerrors.ContextWindowError{
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

	slog.Debug("Llama API response", "response", response)

	slog.Debug("Llama API token usage",
		"input_tokens", response.Usage.PromptTokens,
		"output_tokens", response.Usage.CompletionTokens,
		"total_tokens", response.Usage.TotalTokens)

	return response.Choices[0].Text, nil
}
