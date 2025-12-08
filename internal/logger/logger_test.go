package logger

import (
	"log/slog"
	"testing"

	"release-confidence-score/internal/config"
)

func TestSetup_TextFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "text",
		LogLevel:  "info",
	}

	logger := Setup(cfg)
	if logger == nil {
		t.Fatal("Expected logger, got nil")
	}

	// Verify it's set as default logger
	if slog.Default() != logger {
		t.Error("Logger was not set as default")
	}
}

func TestSetup_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "json",
		LogLevel:  "debug",
	}

	logger := Setup(cfg)
	if logger == nil {
		t.Fatal("Expected logger, got nil")
	}
}

func TestSetup_EmptyFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "",
		LogLevel:  "info",
	}

	logger := Setup(cfg)
	if logger == nil {
		t.Fatal("Expected logger, got nil")
	}
}

func TestSetup_CaseInsensitive(t *testing.T) {
	tests := []struct {
		format string
		level  string
	}{
		{"TEXT", "INFO"},
		{"Json", "Debug"},
		{"JSON", "ERROR"},
		{"text", "warn"},
	}

	for _, tt := range tests {
		t.Run(tt.format+"/"+tt.level, func(t *testing.T) {
			cfg := &config.Config{
				LogFormat: tt.format,
				LogLevel:  tt.level,
			}

			logger := Setup(cfg)
			if logger == nil {
				t.Fatalf("Expected logger for format=%s level=%s, got nil", tt.format, tt.level)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo}, // empty defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLogLevel(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseLogLevel_InvalidDefaultsToInfo(t *testing.T) {
	// Even though this is validated in config.go, test the fallback behavior
	result := parseLogLevel("invalid-level")
	if result != slog.LevelInfo {
		t.Errorf("parseLogLevel(invalid-level) = %v, expected Info", result)
	}
}

func TestSetup_AllLogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := &config.Config{
				LogFormat: "text",
				LogLevel:  level,
			}

			logger := Setup(cfg)
			if logger == nil {
				t.Fatalf("Expected logger for level %s, got nil", level)
			}
		})
	}
}

func TestSetup_BothFormats(t *testing.T) {
	formats := []string{"text", "json"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cfg := &config.Config{
				LogFormat: format,
				LogLevel:  "info",
			}

			logger := Setup(cfg)
			if logger == nil {
				t.Fatalf("Expected logger for format %s, got nil", format)
			}

			// Verify the logger can log without panicking
			logger.Info("test message")
		})
	}
}
