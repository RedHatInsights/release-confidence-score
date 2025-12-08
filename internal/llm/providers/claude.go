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

type ClaudeClient struct {
	config *config.Config
}

type ClaudeRequest struct {
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	Messages         []ClaudeMessage `json:"messages"`
	System           string          `json:"system"`
	Temperature      float64         `json:"temperature"`
}

type ClaudeMessage struct {
	Content []ClaudeContent `json:"content"`
	Role    string          `json:"role"`
}

type ClaudeResponse struct {
	Content []ClaudeContent `json:"content"`
	Usage   ClaudeUsage     `json:"usage"`
}

type ClaudeContent struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func NewClaude(cfg *config.Config) LLMClient {
	return &ClaudeClient{config: cfg}
}

func (c *ClaudeClient) Analyze(userPrompt string) (string, error) {
	cfg := c.config

	// Build Claude-specific endpoint
	endpoint := fmt.Sprintf("%s/sonnet/models/%s:streamRawPredict", cfg.ModelAPI, cfg.ModelID)

	// Create HTTP client
	httpClient := httputil.NewHTTPClient(httputil.HTTPClientOptions{
		Timeout:       time.Duration(cfg.ModelTimeoutSeconds) * time.Second,
		SkipSSLVerify: cfg.ModelSkipSSLVerify,
	})
	req := ClaudeRequest{
		AnthropicVersion: "vertex-2023-10-16",
		System:           system.GetSystemPrompt(cfg),
		Messages: []ClaudeMessage{{
			Role: "user",
			Content: []ClaudeContent{{
				Type: "text",
				Text: userPrompt,
			}},
		}},
		MaxTokens:   cfg.ModelMaxResponseTokens,
		Temperature: 0,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	slog.Debug("Claude API request", "request", jsonData)

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.ModelUserKey)

	slog.Debug("Sending release analysis request to LLM", "provider", "Claude", "model", cfg.ModelID)

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
				Provider:   "Claude",
			}
		}
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response ClaudeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	slog.Debug("Claude API response", "response", response)

	slog.Debug("Claude API token usage",
		"input_tokens", response.Usage.InputTokens,
		"output_tokens", response.Usage.OutputTokens,
		"total_tokens", response.Usage.InputTokens+response.Usage.OutputTokens)

	return response.Content[0].Text, nil
}
