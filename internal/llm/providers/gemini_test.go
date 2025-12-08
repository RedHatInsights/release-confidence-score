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

func TestNewGemini(t *testing.T) {
	cfg := &config.Config{
		ModelProvider: "gemini",
		ModelID:       "gemini-pro",
	}

	client := NewGemini(cfg)

	if client == nil {
		t.Fatal("NewGemini() returned nil")
	}

	geminiClient, ok := client.(*GeminiClient)
	if !ok {
		t.Fatalf("NewGemini() returned wrong type: %T", client)
	}

	if geminiClient.config != cfg {
		t.Error("NewGemini() did not store config correctly")
	}
}

func TestGeminiAnalyze_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Expected Authorization header with Bearer token")
		}

		response := GeminiResponse{
			Choices: []GeminiChoice{
				{
					Message: GeminiMessage{
						Role:    "assistant",
						Content: `{"score": 90, "analysis": "Excellent"}`,
					},
				},
			},
			Usage: GeminiUsage{
				PromptTokens:     150,
				CompletionTokens: 75,
				TotalTokens:      225,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "gemini-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewGemini(cfg)
	result, err := client.Analyze("test prompt")

	if err != nil {
		t.Fatalf("Analyze() unexpected error: %v", err)
	}

	expected := `{"score": 90, "analysis": "Excellent"}`
	if result != expected {
		t.Errorf("Analyze() result = %q, want %q", result, expected)
	}
}

func TestGeminiAnalyze_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GeminiResponse{
			Choices: []GeminiChoice{}, // Empty choices
			Usage: GeminiUsage{
				PromptTokens:     100,
				CompletionTokens: 0,
				TotalTokens:      100,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "gemini-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewGemini(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for empty response, got nil")
	}

	if !strings.Contains(err.Error(), "no choices in response") {
		t.Errorf("Analyze() error = %q, want error containing 'no choices in response'", err.Error())
	}
}

func TestGeminiAnalyze_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway"))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "gemini-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewGemini(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for HTTP 502, got nil")
	}

	if !strings.Contains(err.Error(), "API error 502") {
		t.Errorf("Analyze() error = %q, want error containing 'API error 502'", err.Error())
	}
}

func TestGeminiAnalyze_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json at all"))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "gemini-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewGemini(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("Analyze() error = %q, want error containing 'unmarshal response'", err.Error())
	}
}

func TestGeminiAnalyze_ContextWindowError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "maximum context length exceeded"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "gemini-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewGemini(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Fatal("Analyze() expected error for context window exceeded, got nil")
	}

	// Check if it's a ContextWindowError
	contextErr, ok := err.(*llmerrors.ContextWindowError)
	if !ok {
		t.Fatalf("Analyze() error type = %T, want *llmerrors.ContextWindowError", err)
	}

	if contextErr.Provider != "Gemini" {
		t.Errorf("ContextWindowError.Provider = %q, want %q", contextErr.Provider, "Gemini")
	}
}
