package errors

import (
	"net/http"
	"testing"
)

func TestIsContextWindowError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{
			name:       "Claude prompt too long error",
			statusCode: http.StatusBadRequest,
			body:       `{"type":"error","error":{"type":"invalid_request_error","message":"Prompt is too long: maximum context length exceeded"}}`,
			want:       true,
		},
		{
			name:       "Claude prompt too long - lowercase",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "prompt is too long"}`,
			want:       true,
		},
		{
			name:       "Gemini context length error",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "context length exceeded"}`,
			want:       true,
		},
		{
			name:       "Llama token limit error",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "token limit exceeded"}`,
			want:       true,
		},
		{
			name:       "Generic maximum tokens error",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "maximum tokens exceeded"}`,
			want:       true,
		},
		{
			name:       "Input too large - 413 status",
			statusCode: http.StatusRequestEntityTooLarge,
			body:       `{"error": "input too large"}`,
			want:       true,
		},
		{
			name:       "Rate limit - not context window",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error": "rate limit exceeded"}`,
			want:       false,
		},
		{
			name:       "Wrong status code",
			statusCode: http.StatusInternalServerError,
			body:       `{"error": "prompt is too long"}`,
			want:       false,
		},
		{
			name:       "Generic 400 error",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "invalid request"}`,
			want:       false,
		},
		{
			name:       "Empty body",
			statusCode: http.StatusBadRequest,
			body:       ``,
			want:       false,
		},
		{
			name:       "Context window indicator",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "context window exceeded"}`,
			want:       true,
		},
		{
			name:       "Maximum context indicator",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "maximum context size exceeded"}`,
			want:       true,
		},
		{
			name:       "Too many tokens",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "too many tokens in request"}`,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsContextWindowError(tt.statusCode, []byte(tt.body))
			if got != tt.want {
				t.Errorf("IsContextWindowError() = %v, want %v\nBody: %s", got, tt.want, tt.body)
			}
		})
	}
}

func TestContextWindowError_Error(t *testing.T) {
	err := &ContextWindowError{
		StatusCode: http.StatusBadRequest,
		Message:    "Prompt is too long",
		Provider:   "Claude",
	}

	expected := "context window exceeded for Claude (status 400): Prompt is too long"
	if got := err.Error(); got != expected {
		t.Errorf("ContextWindowError.Error() = %q, want %q", got, expected)
	}
}
