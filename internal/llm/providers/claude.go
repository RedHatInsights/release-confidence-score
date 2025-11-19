package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/llm"
	"release-confidence-score/internal/llm/prompts/system"
	"release-confidence-score/internal/logger"
	"release-confidence-score/internal/shared"
)

type ClaudeClient struct{}

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

type ClaudeErrorResponse struct {
	Type      string      `json:"type"`
	Error     ClaudeError `json:"error"`
	RequestID string      `json:"request_id"`
}

type ClaudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func NewClaude() llm.LLMClient {
	return &ClaudeClient{}
}

func (c *ClaudeClient) Analyze(userPrompt string) (string, error) {
	cfg := config.Get()

	// Build Claude-specific endpoint
	endpoint := fmt.Sprintf("%s/sonnet/models/%s:streamRawPredict", cfg.ModelAPI, cfg.ModelID)

	// Create HTTP client
	httpClient := shared.NewHTTPClient(shared.HTTPClientOptions{
		Timeout:       time.Duration(cfg.TimeoutSeconds) * time.Second,
		SkipSSLVerify: cfg.ModelSkipSSLVerify,
	})
	req := ClaudeRequest{
		AnthropicVersion: "vertex-2023-10-16",
		System:           system.GetSystemPrompt(),
		Messages: []ClaudeMessage{{
			Role: "user",
			Content: []ClaudeContent{{
				Type: "text",
				Text: userPrompt,
			}},
		}},
		MaxTokens:   cfg.MaxResponseTokens,
		Temperature: 0,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	slog.Log(context.Background(), logger.LevelTrace, "Claude API request", "request", jsonData)

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.UserKey)

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
		// Try to parse Claude's structured error response
		var errorResp ClaudeErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			// Check if this is Claude's "Prompt is too long" error
			if errorResp.Type == "error" &&
				errorResp.Error.Type == "invalid_request_error" &&
				strings.Contains(strings.ToLower(errorResp.Error.Message), "prompt is too long") {
				return "", &llm.ContextWindowError{
					StatusCode: resp.StatusCode,
					Message:    errorResp.Error.Message,
					Provider:   "Claude",
				}
			}
		}
		// Not a context window error, return generic error
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response ClaudeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	slog.Log(context.Background(), logger.LevelTrace, "Claude API response", "response", response)

	slog.Debug("Claude API token usage",
		"input_tokens", response.Usage.InputTokens,
		"output_tokens", response.Usage.OutputTokens,
		"total_tokens", response.Usage.InputTokens+response.Usage.OutputTokens)

	return response.Content[0].Text, nil
}
