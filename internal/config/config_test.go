package config

import (
	"os"
	"testing"
)

func TestLoad_ValidConfiguration(t *testing.T) {
	// Set up valid environment
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_GITLAB_BASE_URL", "https://gitlab.example.com")
	t.Setenv("RCS_GITLAB_TOKEN", "gitlab-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")

	cfg, err := Load(false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify config values
	if cfg.GitHubToken != "github-token" {
		t.Errorf("GitHubToken = %v, expected github-token", cfg.GitHubToken)
	}
	if cfg.GitLabToken != "gitlab-token" {
		t.Errorf("GitLabToken = %v, expected gitlab-token", cfg.GitLabToken)
	}
	if cfg.ModelAPI != "https://api.example.com" {
		t.Errorf("ModelAPI = %v, expected https://api.example.com", cfg.ModelAPI)
	}
	if cfg.ModelProvider != "claude" {
		t.Errorf("ModelProvider = %v, expected claude", cfg.ModelProvider)
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	// Set only required fields
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")

	cfg, err := Load(false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify defaults
	if cfg.ModelProvider != "claude" {
		t.Errorf("ModelProvider = %v, expected claude (default)", cfg.ModelProvider)
	}
	if cfg.ModelMaxResponseTokens != 2000 {
		t.Errorf("ModelMaxResponseTokens = %v, expected 2000 (default)", cfg.ModelMaxResponseTokens)
	}
	if cfg.ModelTimeoutSeconds != 120 {
		t.Errorf("ModelTimeoutSeconds = %v, expected 120 (default)", cfg.ModelTimeoutSeconds)
	}
	if cfg.ScoreThresholds.AutoDeploy != 80 {
		t.Errorf("AutoDeploy = %v, expected 80 (default)", cfg.ScoreThresholds.AutoDeploy)
	}
	if cfg.ScoreThresholds.ReviewRequired != 60 {
		t.Errorf("ReviewRequired = %v, expected 60 (default)", cfg.ScoreThresholds.ReviewRequired)
	}
	if cfg.SystemPromptVersion != "v1" {
		t.Errorf("SystemPromptVersion = %v, expected v1 (default)", cfg.SystemPromptVersion)
	}
	if cfg.GitLabSkipSSLVerify != false {
		t.Errorf("GitLabSkipSSLVerify = %v, expected false (default)", cfg.GitLabSkipSSLVerify)
	}
	if cfg.ModelSkipSSLVerify != false {
		t.Errorf("ModelSkipSSLVerify = %v, expected false (default)", cfg.ModelSkipSSLVerify)
	}
}

func TestLoad_MissingModelAPI(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for missing MODEL_API, got none")
	}
	if err.Error() != "RCS_CLAUDE_MODEL_API environment variable is required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_MissingModelID(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for missing MODEL_ID, got none")
	}
	if err.Error() != "RCS_CLAUDE_MODEL_ID environment variable is required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_MissingModelUserKey(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for missing USER_KEY, got none")
	}
	if err.Error() != "RCS_CLAUDE_USER_KEY environment variable is required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_MissingBothGitTokens(t *testing.T) {
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for missing both Git tokens, got none")
	}
	if err.Error() != "at least one of RCS_GITHUB_TOKEN or RCS_GITLAB_TOKEN is required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_GitLabTokenWithoutBaseURL(t *testing.T) {
	t.Setenv("RCS_GITLAB_TOKEN", "gitlab-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for GitLab token without base URL, got none")
	}
	if err.Error() != "RCS_GITLAB_BASE_URL environment variable is required when RCS_GITLAB_TOKEN is provided" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_AppInterfaceModeWithoutGitLabToken(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")

	_, err := Load(true) // app-interface mode
	if err == nil {
		t.Fatal("Expected error for app-interface mode without GitLab token, got none")
	}
	if err.Error() != "RCS_GITLAB_TOKEN environment variable is required for app-interface mode" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_InvalidScoreThresholds(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_SCORE_THRESHOLD_AUTO_DEPLOY", "50")
	t.Setenv("RCS_SCORE_THRESHOLD_REVIEW_REQUIRED", "70")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for invalid score thresholds, got none")
	}
	expected := "RCS_SCORE_THRESHOLD_AUTO_DEPLOY (50) must be greater than or equal to RCS_SCORE_THRESHOLD_REVIEW_REQUIRED (70)"
	if err.Error() != expected {
		t.Errorf("Expected error: %v, got: %v", expected, err)
	}
}

func TestLoad_InvalidMaxResponseTokens(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_MODEL_MAX_RESPONSE_TOKENS", "not-a-number")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for invalid max response tokens, got none")
	}
	if err.Error() != "RCS_MODEL_MAX_RESPONSE_TOKENS must be a valid integer, got: not-a-number" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_InvalidTimeoutSeconds(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_MODEL_TIMEOUT_SECONDS", "invalid")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for invalid timeout seconds, got none")
	}
	if err.Error() != "RCS_MODEL_TIMEOUT_SECONDS must be a valid integer, got: invalid" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_OutOfRangeScoreThreshold(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_SCORE_THRESHOLD_AUTO_DEPLOY", "150")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for out of range score threshold, got none")
	}
	if err.Error() != "RCS_SCORE_THRESHOLD_AUTO_DEPLOY must be between 0 and 100, got: 150" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_InvalidGitLabSkipSSL(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_GITLAB_SKIP_SSL_VERIFY", "not-a-bool")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for invalid GitLab skip SSL, got none")
	}
	if err.Error() != "RCS_GITLAB_SKIP_SSL_VERIFY must be a valid boolean, got: not-a-bool" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_InvalidModelSkipSSL(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_MODEL_SKIP_SSL_VERIFY", "yes")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for invalid model skip SSL, got none")
	}
	if err.Error() != "RCS_MODEL_SKIP_SSL_VERIFY must be a valid boolean, got: yes" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_ValidBooleanValues(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_GITLAB_SKIP_SSL_VERIFY", "true")
	t.Setenv("RCS_MODEL_SKIP_SSL_VERIFY", "1")

	cfg, err := Load(false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !cfg.GitLabSkipSSLVerify {
		t.Error("GitLabSkipSSLVerify should be true")
	}
	if !cfg.ModelSkipSSLVerify {
		t.Error("ModelSkipSSLVerify should be true")
	}
}

func TestLoad_DifferentModelProvider(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_MODEL_PROVIDER", "gemini")
	t.Setenv("RCS_GEMINI_MODEL_API", "https://gemini.example.com")
	t.Setenv("RCS_GEMINI_MODEL_ID", "gemini-model")
	t.Setenv("RCS_GEMINI_USER_KEY", "gemini-key")

	cfg, err := Load(false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.ModelProvider != "gemini" {
		t.Errorf("ModelProvider = %v, expected gemini", cfg.ModelProvider)
	}
	if cfg.ModelAPI != "https://gemini.example.com" {
		t.Errorf("ModelAPI = %v, expected https://gemini.example.com", cfg.ModelAPI)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		setValue string
		setVar   bool
		defVal   string
		expected string
	}{
		{
			name:     "env var set",
			key:      "TEST_VAR_1",
			setValue: "custom-value",
			setVar:   true,
			defVal:   "default-value",
			expected: "custom-value",
		},
		{
			name:     "env var not set",
			key:      "TEST_VAR_2",
			setVar:   false,
			defVal:   "default-value",
			expected: "default-value",
		},
		{
			name:     "env var set to empty",
			key:      "TEST_VAR_3",
			setValue: "",
			setVar:   true,
			defVal:   "default-value",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setVar {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvOrDefault(tt.key, tt.defVal)
			if result != tt.expected {
				t.Errorf("getEnvOrDefault(%s, %s) = %s, expected %s", tt.key, tt.defVal, result, tt.expected)
			}
		})
	}
}

func TestParseIntEnvOrDefault(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		setValue  string
		setVar    bool
		defVal    int
		min       int
		max       int
		expected  int
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid value",
			key:       "TEST_INT_1",
			setValue:  "50",
			setVar:    true,
			defVal:    10,
			min:       0,
			max:       100,
			expected:  50,
			expectErr: false,
		},
		{
			name:      "not set uses default",
			key:       "TEST_INT_2",
			setVar:    false,
			defVal:    10,
			min:       0,
			max:       100,
			expected:  10,
			expectErr: false,
		},
		{
			name:      "invalid integer",
			key:       "TEST_INT_3",
			setValue:  "not-a-number",
			setVar:    true,
			defVal:    10,
			min:       0,
			max:       100,
			expectErr: true,
			errMsg:    "TEST_INT_3 must be a valid integer, got: not-a-number",
		},
		{
			name:      "below minimum",
			key:       "TEST_INT_4",
			setValue:  "-5",
			setVar:    true,
			defVal:    10,
			min:       0,
			max:       100,
			expectErr: true,
			errMsg:    "TEST_INT_4 must be between 0 and 100, got: -5",
		},
		{
			name:      "above maximum",
			key:       "TEST_INT_5",
			setValue:  "150",
			setVar:    true,
			defVal:    10,
			min:       0,
			max:       100,
			expectErr: true,
			errMsg:    "TEST_INT_5 must be between 0 and 100, got: 150",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setVar {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}

			result, err := parseIntEnvOrDefault(tt.key, tt.defVal, tt.min, tt.max)

			if tt.expectErr {
				if err == nil {
					t.Fatal("Expected error, got none")
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Expected error: %s, got: %s", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_LOG_LEVEL", "invalid")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for invalid log level, got none")
	}
	if err.Error() != "RCS_LOG_LEVEL must be one of: [debug info warn error]; got: invalid" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_InvalidLogFormat(t *testing.T) {
	t.Setenv("RCS_GITHUB_TOKEN", "github-token")
	t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
	t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
	t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
	t.Setenv("RCS_LOG_FORMAT", "xml")

	_, err := Load(false)
	if err == nil {
		t.Fatal("Expected error for invalid log format, got none")
	}
	if err.Error() != "RCS_LOG_FORMAT must be one of: [text json]; got: xml" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoad_ValidLogLevels(t *testing.T) {
	// Test both lowercase (from validLogLevels) and uppercase variants
	testLevels := append(validLogLevels, "DEBUG", "INFO", "WARN", "ERROR")

	for _, level := range testLevels {
		t.Run(level, func(t *testing.T) {
			t.Setenv("RCS_GITHUB_TOKEN", "github-token")
			t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
			t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
			t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
			t.Setenv("RCS_LOG_LEVEL", level)

			_, err := Load(false)
			if err != nil {
				t.Fatalf("Expected no error for log level %s, got: %v", level, err)
			}
		})
	}
}

func TestLoad_ValidLogFormats(t *testing.T) {
	// Test both lowercase (from validLogFormats) and uppercase variants
	testFormats := append(validLogFormats, "TEXT", "JSON")

	for _, format := range testFormats {
		t.Run(format, func(t *testing.T) {
			t.Setenv("RCS_GITHUB_TOKEN", "github-token")
			t.Setenv("RCS_CLAUDE_MODEL_API", "https://api.example.com")
			t.Setenv("RCS_CLAUDE_MODEL_ID", "claude-model")
			t.Setenv("RCS_CLAUDE_USER_KEY", "api-key")
			t.Setenv("RCS_LOG_FORMAT", format)

			_, err := Load(false)
			if err != nil {
				t.Fatalf("Expected no error for log format %s, got: %v", format, err)
			}
		})
	}
}

func TestParseBoolEnvOrDefault(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		setValue  string
		setVar    bool
		defVal    bool
		expected  bool
		expectErr bool
		errMsg    string
	}{
		{
			name:      "true value",
			key:       "TEST_BOOL_1",
			setValue:  "true",
			setVar:    true,
			defVal:    false,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "false value",
			key:       "TEST_BOOL_2",
			setValue:  "false",
			setVar:    true,
			defVal:    true,
			expected:  false,
			expectErr: false,
		},
		{
			name:      "1 as true",
			key:       "TEST_BOOL_3",
			setValue:  "1",
			setVar:    true,
			defVal:    false,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "0 as false",
			key:       "TEST_BOOL_4",
			setValue:  "0",
			setVar:    true,
			defVal:    true,
			expected:  false,
			expectErr: false,
		},
		{
			name:      "not set uses default false",
			key:       "TEST_BOOL_5",
			setVar:    false,
			defVal:    false,
			expected:  false,
			expectErr: false,
		},
		{
			name:      "not set uses default true",
			key:       "TEST_BOOL_6",
			setVar:    false,
			defVal:    true,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "invalid boolean",
			key:       "TEST_BOOL_7",
			setValue:  "invalid",
			setVar:    true,
			defVal:    false,
			expectErr: true,
			errMsg:    "TEST_BOOL_7 must be a valid boolean, got: invalid",
		},
		{
			name:      "yes is invalid",
			key:       "TEST_BOOL_8",
			setValue:  "yes",
			setVar:    true,
			defVal:    false,
			expectErr: true,
			errMsg:    "TEST_BOOL_8 must be a valid boolean, got: yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setVar {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}

			result, err := parseBoolEnvOrDefault(tt.key, tt.defVal)

			if tt.expectErr {
				if err == nil {
					t.Fatal("Expected error, got none")
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Expected error: %s, got: %s", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}
