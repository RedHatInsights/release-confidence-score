package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"release-confidence-score/internal/config"
	llmerrors "release-confidence-score/internal/llm/errors"
)

func TestNewClaude(t *testing.T) {
	cfg := &config.Config{
		ModelProvider: "claude",
		ModelID:       "claude-3-sonnet",
	}

	client := NewClaude(cfg)

	if client == nil {
		t.Fatal("NewClaude() returned nil")
	}

	claudeClient, ok := client.(*ClaudeClient)
	if !ok {
		t.Fatalf("NewClaude() returned wrong type: %T", client)
	}

	if claudeClient.config != cfg {
		t.Error("NewClaude() did not store config correctly")
	}
}

func TestClaudeAnalyze_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Expected Authorization header with Bearer token")
		}

		// Send successful response
		response := ClaudeResponse{
			Content: []ClaudeContent{
				{Type: "text", Text: `{"score": 85, "analysis": "Good"}`},
			},
			Usage: ClaudeUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "claude-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewClaude(cfg)
	result, err := client.Analyze("test prompt")

	if err != nil {
		t.Fatalf("Analyze() unexpected error: %v", err)
	}

	expected := `{"score": 85, "analysis": "Good"}`
	if result != expected {
		t.Errorf("Analyze() result = %q, want %q", result, expected)
	}
}

func TestClaudeAnalyze_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ClaudeResponse{
			Content: []ClaudeContent{}, // Empty content
			Usage: ClaudeUsage{
				InputTokens:  100,
				OutputTokens: 0,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "claude-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewClaude(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for empty response, got nil")
	}

	if !strings.Contains(err.Error(), "no content in response") {
		t.Errorf("Analyze() error = %q, want error containing 'no content in response'", err.Error())
	}
}

func TestClaudeAnalyze_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "claude-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewClaude(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for HTTP 500, got nil")
	}

	if !strings.Contains(err.Error(), "API error 500") {
		t.Errorf("Analyze() error = %q, want error containing 'API error 500'", err.Error())
	}
}

func TestClaudeAnalyze_ContextWindowError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a simple error message with context window indicator
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "prompt is too long: maximum context length exceeded"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "claude-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewClaude(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Fatal("Analyze() expected error for context window exceeded, got nil")
	}

	// Check if it's a ContextWindowError
	contextErr, ok := err.(*llmerrors.ContextWindowError)
	if !ok {
		t.Fatalf("Analyze() error type = %T, want *llmerrors.ContextWindowError", err)
	}

	if contextErr.Provider != "Claude" {
		t.Errorf("ContextWindowError.Provider = %q, want %q", contextErr.Provider, "Claude")
	}

	if contextErr.StatusCode != http.StatusBadRequest {
		t.Errorf("ContextWindowError.StatusCode = %d, want %d", contextErr.StatusCode, http.StatusBadRequest)
	}
}

func TestClaudeAnalyze_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "claude-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewClaude(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("Analyze() error = %q, want error containing 'unmarshal response'", err.Error())
	}
}
