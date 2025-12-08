package errors

import (
	"fmt"
	"strings"
)

// ContextWindowError represents an error when the LLM's context window is exceeded
type ContextWindowError struct {
	StatusCode int
	Message    string
	Provider   string
}

func (e *ContextWindowError) Error() string {
	return fmt.Sprintf("context window exceeded for %s (status %d): %s", e.Provider, e.StatusCode, e.Message)
}

// IsContextWindowError checks if an HTTP response indicates a context window error
func IsContextWindowError(statusCode int, body []byte) bool {
	// Check status codes that typically indicate payload/context issues
	if statusCode != 400 && statusCode != 413 && statusCode != 429 {
		return false
	}

	// Parse response body for context window error indicators
	bodyStr := strings.ToLower(string(body))

	contextWindowIndicators := []string{
		"context length",
		"context window",
		"token limit",
		"maximum context",
		"input too large",
		"prompt is too long",
		"prompt too long",
		"maximum tokens",
		"exceeds maximum",
		"too many tokens",
	}

	for _, indicator := range contextWindowIndicators {
		if strings.Contains(bodyStr, indicator) {
			return true
		}
	}

	return false
}
