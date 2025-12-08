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

func TestNewLlama(t *testing.T) {
	cfg := &config.Config{
		ModelProvider: "llama",
		ModelID:       "llama-3",
	}

	client := NewLlama(cfg)

	if client == nil {
		t.Fatal("NewLlama() returned nil")
	}

	llamaClient, ok := client.(*LlamaClient)
	if !ok {
		t.Fatalf("NewLlama() returned wrong type: %T", client)
	}

	if llamaClient.config != cfg {
		t.Error("NewLlama() did not store config correctly")
	}
}

func TestLlamaAnalyze_Success(t *testing.T) {
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

		response := LlamaResponse{
			Choices: []LlamaChoice{
				{
					Text: `{"score": 88, "analysis": "Very good"}`,
				},
			},
			Usage: LlamaUsage{
				PromptTokens:     120,
				CompletionTokens: 60,
				TotalTokens:      180,
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "llama-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewLlama(cfg)
	result, err := client.Analyze("test prompt")

	if err != nil {
		t.Fatalf("Analyze() unexpected error: %v", err)
	}

	expected := `{"score": 88, "analysis": "Very good"}`
	if result != expected {
		t.Errorf("Analyze() result = %q, want %q", result, expected)
	}
}

func TestLlamaAnalyze_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LlamaResponse{
			Choices: []LlamaChoice{}, // Empty choices
			Usage: LlamaUsage{
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
		ModelID:                "llama-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewLlama(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for empty response, got nil")
	}

	if !strings.Contains(err.Error(), "no choices in response") {
		t.Errorf("Analyze() error = %q, want error containing 'no choices in response'", err.Error())
	}
}

func TestLlamaAnalyze_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "llama-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewLlama(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for HTTP 503, got nil")
	}

	if !strings.Contains(err.Error(), "API error 503") {
		t.Errorf("Analyze() error = %q, want error containing 'API error 503'", err.Error())
	}
}

func TestLlamaAnalyze_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{invalid json response"))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "llama-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewLlama(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Error("Analyze() expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("Analyze() error = %q, want error containing 'unmarshal response'", err.Error())
	}
}

func TestLlamaAnalyze_ContextWindowError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "This model's maximum context length is 4096 tokens"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		ModelAPI:               server.URL,
		ModelID:                "llama-test",
		ModelUserKey:           "test-key",
		ModelTimeoutSeconds:    30,
		ModelMaxResponseTokens: 1000,
		SystemPromptVersion:    "v1",
	}

	client := NewLlama(cfg)
	_, err := client.Analyze("test prompt")

	if err == nil {
		t.Fatal("Analyze() expected error for context window exceeded, got nil")
	}

	// Check if it's a ContextWindowError
	contextErr, ok := err.(*llmerrors.ContextWindowError)
	if !ok {
		t.Fatalf("Analyze() error type = %T, want *llmerrors.ContextWindowError", err)
	}

	if contextErr.Provider != "Llama" {
		t.Errorf("ContextWindowError.Provider = %q, want %q", contextErr.Provider, "Llama")
	}
}
