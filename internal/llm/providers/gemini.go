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

type GeminiClient struct{}

type GeminiRequest struct {
	MaxTokens   int             `json:"max_tokens"`
	Messages    []GeminiMessage `json:"messages"`
	Model       string          `json:"model"`
	Temperature float64         `json:"temperature"`
}

type GeminiMessage struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type GeminiResponse struct {
	Choices []GeminiChoice `json:"choices"`
	Usage   GeminiUsage    `json:"usage"`
}

type GeminiChoice struct {
	Message GeminiMessage `json:"message"`
}

type GeminiUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func NewGemini() llm.LLMClient {
	return &GeminiClient{}
}

func (g *GeminiClient) Analyze(userPrompt string) (string, error) {
	cfg := config.Get()

	// Gemini uses combined prompt
	combinedPrompt := system.GetSystemPrompt() + "\n\n" + userPrompt

	req := GeminiRequest{
		Model: cfg.ModelID,
		Messages: []GeminiMessage{{
			Role:    "user",
			Content: combinedPrompt,
		}},
		MaxTokens:   cfg.MaxResponseTokens,
		Temperature: 0,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	slog.Log(context.Background(), logger.LevelTrace, "Gemini API request", "request", jsonData)

	url := cfg.ModelAPI + "/v1beta/openai/chat/completions"
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

	slog.Debug("Sending release analysis request to LLM", "provider", "Gemini", "model", cfg.ModelID)

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
				Provider:   "Gemini",
			}
		}
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response GeminiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	slog.Log(context.Background(), logger.LevelTrace, "Gemini API response", "response", response)

	slog.Debug("Gemini API token usage",
		"input_tokens", response.Usage.PromptTokens,
		"output_tokens", response.Usage.CompletionTokens,
		"total_tokens", response.Usage.TotalTokens)

	return response.Choices[0].Message.Content, nil
}
