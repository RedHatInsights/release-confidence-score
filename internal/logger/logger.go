package logger

import (
	"log/slog"
	"os"
	"strings"

	"release-confidence-score/internal/config"
)

const (
	// LevelTrace is a custom log level below Debug for very verbose logging
	LevelTrace = slog.Level(-8)
)

// Setup initializes and configures the application logger
func Setup() *slog.Logger {
	cfg := config.Get()
	level := parseLogLevel(cfg.LogLevel)

	var handler slog.Handler
	switch strings.ToLower(cfg.LogFormat) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	default: // "text" or empty
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := slog.New(handler)

	// Set as default logger for the entire application
	slog.SetDefault(logger)

	return logger
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo // Default to info level
	}
}
