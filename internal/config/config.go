package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// valid log formats and levels
var (
	validLogFormats = []string{"text", "json"}
	validLogLevels  = []string{"debug", "info", "warn", "error"}
)

type Config struct {
	GitHubToken            string
	GitLabBaseURL          string
	GitLabSkipSSLVerify    bool
	GitLabToken            string
	LogFormat              string
	LogLevel               string
	ModelAPI               string
	ModelID                string
	ModelMaxResponseTokens int
	ModelProvider          string
	ModelSkipSSLVerify     bool
	ModelTimeoutSeconds    int
	ModelUserKey           string
	ScoreThresholds        ScoreThresholds
	SystemPromptVersion    string
}

type ScoreThresholds struct {
	AutoDeploy     int // Score above which auto-deploy is recommended
	ReviewRequired int // Score below which manual review is required
}

// Load creates a new Config instance from environment variables and validates it
func Load(isAppInterfaceMode bool) (*Config, error) {

	// Parse Git platform configuration
	gitHubToken := os.Getenv("RCS_GITHUB_TOKEN")
	gitLabBaseURL := os.Getenv("RCS_GITLAB_BASE_URL")
	gitLabToken := os.Getenv("RCS_GITLAB_TOKEN")

	gitLabSkipSSL, err := parseBoolEnvOrDefault("RCS_GITLAB_SKIP_SSL_VERIFY", false)
	if err != nil {
		return nil, err
	}

	// Parse logging configuration
	logFormat := os.Getenv("RCS_LOG_FORMAT")
	logLevel := os.Getenv("RCS_LOG_LEVEL")

	// Parse model configuration
	modelProvider := getEnvOrDefault("RCS_MODEL_PROVIDER", "claude")
	prefix := strings.ToUpper(modelProvider)
	modelAPI := os.Getenv(fmt.Sprintf("RCS_%s_MODEL_API", prefix))
	modelID := os.Getenv(fmt.Sprintf("RCS_%s_MODEL_ID", prefix))
	modelUserKey := os.Getenv(fmt.Sprintf("RCS_%s_USER_KEY", prefix))

	modelSkipSSL, err := parseBoolEnvOrDefault("RCS_MODEL_SKIP_SSL_VERIFY", false)
	if err != nil {
		return nil, err
	}

	modelMaxResponseTokens, err := parseIntEnvOrDefault("RCS_MODEL_MAX_RESPONSE_TOKENS", 2000, 1, 1000000000)
	if err != nil {
		return nil, err
	}
	modelTimeoutSeconds, err := parseIntEnvOrDefault("RCS_MODEL_TIMEOUT_SECONDS", 120, 1, 1000000000)
	if err != nil {
		return nil, err
	}

	// Parse score thresholds
	autoDeploy, err := parseIntEnvOrDefault("RCS_SCORE_THRESHOLD_AUTO_DEPLOY", 80, 0, 100)
	if err != nil {
		return nil, err
	}
	reviewRequired, err := parseIntEnvOrDefault("RCS_SCORE_THRESHOLD_REVIEW_REQUIRED", 60, 0, 100)
	if err != nil {
		return nil, err
	}

	// Parse system prompt version
	systemPromptVersion := getEnvOrDefault("RCS_SYSTEM_PROMPT_VERSION", "v1")

	// Build config struct
	cfg := &Config{
		GitHubToken:            gitHubToken,
		GitLabBaseURL:          gitLabBaseURL,
		GitLabSkipSSLVerify:    gitLabSkipSSL,
		GitLabToken:            gitLabToken,
		LogFormat:              logFormat,
		LogLevel:               logLevel,
		ModelAPI:               modelAPI,
		ModelID:                modelID,
		ModelMaxResponseTokens: modelMaxResponseTokens,
		ModelProvider:          modelProvider,
		ModelSkipSSLVerify:     modelSkipSSL,
		ModelTimeoutSeconds:    modelTimeoutSeconds,
		ModelUserKey:           modelUserKey,
		ScoreThresholds: ScoreThresholds{
			AutoDeploy:     autoDeploy,
			ReviewRequired: reviewRequired,
		},
		SystemPromptVersion: systemPromptVersion,
	}

	// Validate configuration
	if err := validateConfig(cfg, isAppInterfaceMode, prefix); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

// parseIntEnvOrDefault parses an integer environment variable with range validation or returns a default value if not set
func parseIntEnvOrDefault(key string, defaultVal, min, max int) (int, error) {
	str, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal, nil
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer, got: %s", key, str)
	}

	if val < min || val > max {
		return 0, fmt.Errorf("%s must be between %d and %d, got: %d", key, min, max, val)
	}

	return val, nil
}

// parseBoolEnvOrDefault parses a boolean environment variable or returns a default value if not set
func parseBoolEnvOrDefault(key string, defaultVal bool) (bool, error) {
	str, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal, nil
	}

	val, err := strconv.ParseBool(str)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean, got: %s", key, str)
	}

	return val, nil
}

// validateConfig performs all validation on the loaded configuration
func validateConfig(cfg *Config, isAppInterfaceMode bool, modelProviderPrefix string) error {

	// Validate Git platform configuration
	if cfg.GitHubToken == "" && cfg.GitLabToken == "" {
		return fmt.Errorf("at least one of RCS_GITHUB_TOKEN or RCS_GITLAB_TOKEN is required")
	}
	if isAppInterfaceMode && cfg.GitLabToken == "" {
		return fmt.Errorf("RCS_GITLAB_TOKEN environment variable is required for app-interface mode")
	}
	if cfg.GitLabToken != "" && cfg.GitLabBaseURL == "" {
		return fmt.Errorf("RCS_GITLAB_BASE_URL environment variable is required when RCS_GITLAB_TOKEN is provided")
	}

	// Validate logging configuration
	if cfg.LogFormat != "" {
		if !slices.Contains(validLogFormats, strings.ToLower(cfg.LogFormat)) {
			return fmt.Errorf("RCS_LOG_FORMAT must be one of: %v; got: %s", validLogFormats, cfg.LogFormat)
		}
	}
	if cfg.LogLevel != "" {
		if !slices.Contains(validLogLevels, strings.ToLower(cfg.LogLevel)) {
			return fmt.Errorf("RCS_LOG_LEVEL must be one of: %v; got: %s", validLogLevels, cfg.LogLevel)
		}
	}

	// Validate required model configuration
	if cfg.ModelAPI == "" {
		return fmt.Errorf("RCS_%s_MODEL_API environment variable is required", modelProviderPrefix)
	}
	if cfg.ModelID == "" {
		return fmt.Errorf("RCS_%s_MODEL_ID environment variable is required", modelProviderPrefix)
	}
	if cfg.ModelUserKey == "" {
		return fmt.Errorf("RCS_%s_USER_KEY environment variable is required", modelProviderPrefix)
	}

	// Validate score threshold logic
	if cfg.ScoreThresholds.AutoDeploy < cfg.ScoreThresholds.ReviewRequired {
		return fmt.Errorf("RCS_SCORE_THRESHOLD_AUTO_DEPLOY (%d) must be greater than or equal to RCS_SCORE_THRESHOLD_REVIEW_REQUIRED (%d)",
			cfg.ScoreThresholds.AutoDeploy, cfg.ScoreThresholds.ReviewRequired)
	}

	return nil
}
