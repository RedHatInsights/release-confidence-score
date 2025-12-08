package system

import (
	"strings"
	"testing"

	"release-confidence-score/internal/config"
)

func TestGetSystemPrompt(t *testing.T) {
	tests := []struct {
		name            string
		version         string
		expectV1        bool
		expectV2        bool
		expectFallback  bool
		allowEmptyForV2 bool
	}{
		{
			name:     "v1 version",
			version:  "v1",
			expectV1: true,
		},
		{
			name:            "v2 version",
			version:         "v2",
			expectV2:        true,
			allowEmptyForV2: true, // v2 is currently a placeholder/empty
		},
		{
			name:           "unknown version",
			version:        "v3",
			expectV1:       true,
			expectFallback: true,
		},
		{
			name:           "empty version",
			version:        "",
			expectV1:       true,
			expectFallback: true,
		},
		{
			name:           "invalid version",
			version:        "invalid",
			expectV1:       true,
			expectFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				SystemPromptVersion: tt.version,
			}

			result := GetSystemPrompt(cfg)

			// Verify result is not empty (unless it's v2 which is allowed to be empty for now)
			if result == "" && !tt.allowEmptyForV2 {
				t.Error("GetSystemPrompt() returned empty string")
			}

			// Verify we got the expected version
			if tt.expectV1 {
				if result != systemPromptV1 {
					t.Error("GetSystemPrompt() did not return v1 prompt")
				}
			}

			if tt.expectV2 {
				if result != systemPromptV2 {
					t.Error("GetSystemPrompt() did not return v2 prompt")
				}
			}
		})
	}
}

func TestEmbeddedPromptsNotEmpty(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		allowEmpty bool
	}{
		{"systemPromptV1", systemPromptV1, false},
		{"systemPromptV2", systemPromptV2, true}, // v2 is currently a placeholder
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prompt == "" && !tt.allowEmpty {
				t.Errorf("%s is empty", tt.name)
			}

			// Verify it looks like a valid prompt (has some content) if not allowed to be empty
			if !tt.allowEmpty && len(tt.prompt) < 10 {
				t.Errorf("%s is too short (%d chars)", tt.name, len(tt.prompt))
			}
		})
	}
}

func TestEmbeddedPromptsAreDistinct(t *testing.T) {
	if systemPromptV1 == systemPromptV2 {
		t.Error("systemPromptV1 and systemPromptV2 should be different")
	}
}

func TestSystemPromptVersion1Content(t *testing.T) {
	// Verify v1 prompt contains expected content
	// This ensures the embed directive is working correctly
	if !strings.Contains(systemPromptV1, "You are") && !strings.Contains(systemPromptV1, "#") {
		t.Error("systemPromptV1 does not appear to contain valid prompt content")
	}
}

func TestSystemPromptVersion2Content(t *testing.T) {
	// v2 is currently a placeholder/empty file
	// This test verifies the embed directive is working (it should be empty)
	// When v2 is implemented, this test should be updated to check for content
	if systemPromptV2 != "" && !strings.Contains(systemPromptV2, "You are") && !strings.Contains(systemPromptV2, "#") {
		t.Error("systemPromptV2 does not appear to contain valid prompt content")
	}
}

func TestGetSystemPromptConsistency(t *testing.T) {
	// Verify that calling GetSystemPrompt multiple times returns the same result
	cfg := &config.Config{
		SystemPromptVersion: "v1",
	}

	result1 := GetSystemPrompt(cfg)
	result2 := GetSystemPrompt(cfg)

	if result1 != result2 {
		t.Error("GetSystemPrompt() returned different results for same config")
	}
}

func TestGetSystemPromptSwitchingVersions(t *testing.T) {
	// Test switching between versions
	cfgV1 := &config.Config{SystemPromptVersion: "v1"}
	cfgV2 := &config.Config{SystemPromptVersion: "v2"}

	resultV1 := GetSystemPrompt(cfgV1)
	resultV2 := GetSystemPrompt(cfgV2)

	// v1 and v2 can be different (v2 might be empty currently)
	// Just verify they return the correct prompt variables
	if resultV1 != systemPromptV1 {
		t.Error("GetSystemPrompt(v1) did not return v1 prompt")
	}

	if resultV2 != systemPromptV2 {
		t.Error("GetSystemPrompt(v2) did not return v2 prompt")
	}
}
