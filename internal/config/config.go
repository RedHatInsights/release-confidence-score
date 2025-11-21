package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type Config struct {
	GitHubToken         string
	GitLabBaseURL       string
	GitLabSkipSSLVerify bool
	GitLabToken         string
	ModelAPI            string
	ModelID             string
	ModelProvider       string
	UserKey             string
	ModelSkipSSLVerify  bool
	MaxResponseTokens   int
	TimeoutSeconds      int
	LogLevel            string
	LogFormat           string
	SystemPromptVersion string
	ScoreThresholds     ScoreThresholds
}

type ScoreThresholds struct {
	AutoDeploy     int // Score above which auto-deploy is recommended
	ReviewRequired int // Score below which manual review is required
}

var (
	instance *Config
	once     sync.Once
)

func Get() *Config {
	once.Do(func() {
		var err error
		instance, err = load()
		if err != nil {
			panic("Failed to load configuration: " + err.Error())
		}
	})
	return instance
}

// Reset clears the singleton configuration, allowing it to be reloaded.
// This function can only be called during tests.
func Reset() {
	if !testing.Testing() {
		panic("Reset() can only be called during tests")
	}
	once = sync.Once{}
	instance = nil
}

func load() (*Config, error) {
	// Parse max response tokens with default
	maxResponseTokens := 2000 // Default for LLM response length
	if maxResponseTokensStr := os.Getenv("MODEL_MAX_RESPONSE_TOKENS"); maxResponseTokensStr != "" {
		if parsed, err := strconv.Atoi(maxResponseTokensStr); err == nil && parsed > 0 {
			maxResponseTokens = parsed
		}
	}

	// Parse timeout with default
	timeoutSeconds := 120 // Default 2 minute timeout
	if timeoutStr := os.Getenv("MODEL_TIMEOUT_SECONDS"); timeoutStr != "" {
		if parsed, err := strconv.Atoi(timeoutStr); err == nil && parsed > 0 {
			timeoutSeconds = parsed
		}
	}

	// Parse score thresholds with defaults
	scoreThresholds := ScoreThresholds{
		AutoDeploy:     80, // Default: scores 80+ are safe for auto-deploy
		ReviewRequired: 60, // Default: scores below 60 require manual review
	}

	if autoDeployStr := os.Getenv("SCORE_THRESHOLD_AUTO_DEPLOY"); autoDeployStr != "" {
		if parsed, err := strconv.Atoi(autoDeployStr); err == nil && parsed >= 0 && parsed <= 100 {
			scoreThresholds.AutoDeploy = parsed
		}
	}

	if reviewRequiredStr := os.Getenv("SCORE_THRESHOLD_REVIEW_REQUIRED"); reviewRequiredStr != "" {
		if parsed, err := strconv.Atoi(reviewRequiredStr); err == nil && parsed >= 0 && parsed <= 100 {
			scoreThresholds.ReviewRequired = parsed
		}
	}

	// Get model provider from env var with default
	modelProvider := os.Getenv("MODEL_PROVIDER")
	if modelProvider == "" {
		modelProvider = "claude" // default
	}

	// Get system prompt version from env var with default
	systemPromptVersion := os.Getenv("SYSTEM_PROMPT_VERSION")
	if systemPromptVersion == "" {
		systemPromptVersion = "v1" // default
	}

	// Get provider-specific configuration using prefixed environment variables
	prefix := strings.ToUpper(modelProvider)
	modelAPI := os.Getenv(prefix + "_MODEL_API")
	modelID := os.Getenv(prefix + "_MODEL_ID")
	userKey := os.Getenv(prefix + "_USER_KEY")

	// Validate required fields
	if modelAPI == "" {
		return nil, fmt.Errorf("%s_MODEL_API environment variable is required", prefix)
	}
	if modelID == "" {
		return nil, fmt.Errorf("%s_MODEL_ID environment variable is required", prefix)
	}
	if userKey == "" {
		return nil, fmt.Errorf("%s_USER_KEY environment variable is required", prefix)
	}

	cfg := &Config{
		GitHubToken:         os.Getenv("GITHUB_TOKEN"),
		GitLabBaseURL:       os.Getenv("GITLAB_BASE_URL"),
		GitLabSkipSSLVerify: os.Getenv("GITLAB_SKIP_SSL_VERIFY") == "true",
		GitLabToken:         os.Getenv("GITLAB_TOKEN"),
		ModelAPI:            modelAPI,
		ModelID:             modelID,
		ModelProvider:       modelProvider,
		UserKey:             userKey,
		ModelSkipSSLVerify:  os.Getenv("MODEL_SKIP_SSL_VERIFY") == "true",
		MaxResponseTokens:   maxResponseTokens,
		TimeoutSeconds:      timeoutSeconds,
		LogLevel:            os.Getenv("LOG_LEVEL"),
		LogFormat:           os.Getenv("LOG_FORMAT"),
		SystemPromptVersion: systemPromptVersion,
		ScoreThresholds:     scoreThresholds,
	}

	if cfg.GitHubToken == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	if cfg.GitLabBaseURL == "" {
		return nil, fmt.Errorf("GITLAB_BASE_URL environment variable is required")
	}

	if cfg.GitLabToken == "" {
		return nil, fmt.Errorf("GITLAB_TOKEN environment variable is required")
	}

	return cfg, nil
}
