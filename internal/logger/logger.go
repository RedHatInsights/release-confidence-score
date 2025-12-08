package logger

import (
	"log/slog"
	"os"
	"strings"

	"release-confidence-score/internal/config"
)

// Setup initializes and configures the application logger
func Setup(cfg *config.Config) *slog.Logger {
	level := parseLogLevel(cfg.LogLevel)
	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.LogFormat) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	default: // "text" or empty (already validated in config.go)
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	}

	logger := slog.New(handler)

	// Set as default logger for the entire application
	slog.SetDefault(logger)

	return logger
}

// parseLogLevel converts string log level to slog.Level
// Note: Input is validated in config.go, so only valid values reach this function
func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "info", "": // empty defaults to info
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// Should never reach here due to validation in config.go
		return slog.LevelInfo
	}
}
