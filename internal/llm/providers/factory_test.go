package providers

import (
	"fmt"
	"testing"

	"release-confidence-score/internal/config"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		expectError bool
		expectType  string
	}{
		{
			name:        "claude provider",
			provider:    "claude",
			expectError: false,
			expectType:  "*providers.ClaudeClient",
		},
		{
			name:        "gemini provider",
			provider:    "gemini",
			expectError: false,
			expectType:  "*providers.GeminiClient",
		},
		{
			name:        "llama provider",
			provider:    "llama",
			expectError: false,
			expectType:  "*providers.LlamaClient",
		},
		{
			name:        "unsupported provider",
			provider:    "openai",
			expectError: true,
		},
		{
			name:        "empty provider",
			provider:    "",
			expectError: true,
		},
		{
			name:        "invalid provider",
			provider:    "invalid-provider-123",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ModelProvider: tt.provider,
			}

			client, err := NewClient(cfg)

			if tt.expectError {
				if err == nil {
					t.Errorf("NewClient() expected error for provider %q, got nil", tt.provider)
				}
				if client != nil {
					t.Errorf("NewClient() expected nil client for error case, got %T", client)
				}
			} else {
				if err != nil {
					t.Errorf("NewClient() unexpected error: %v", err)
				}
				if client == nil {
					t.Error("NewClient() returned nil client")
				}

				// Verify correct type
				clientType := fmt.Sprintf("%T", client)
				if clientType != tt.expectType {
					t.Errorf("NewClient() type = %s, want %s", clientType, tt.expectType)
				}
			}
		})
	}
}

func TestNewClientErrorMessage(t *testing.T) {
	cfg := &config.Config{
		ModelProvider: "unsupported-provider",
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Fatal("NewClient() expected error, got nil")
	}

	expectedMsg := "unsupported model provider: unsupported-provider"
	if err.Error() != expectedMsg {
		t.Errorf("NewClient() error message = %q, want %q", err.Error(), expectedMsg)
	}
}
