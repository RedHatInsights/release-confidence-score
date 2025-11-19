package shared

import (
	"testing"
)

func TestParseUserGuidance(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedText  string
		expectedFound bool
	}{
		{
			name:          "simple guidance",
			input:         "/rcs this is a guidance message",
			expectedText:  "this is a guidance message",
			expectedFound: true,
		},
		{
			name:          "case insensitive",
			input:         "/RCS This Is A Message",
			expectedText:  "This Is A Message",
			expectedFound: true,
		},
		{
			name:          "multiline guidance",
			input:         "/rcs first line\nsecond line\nthird line",
			expectedText:  "first line\nsecond line\nthird line",
			expectedFound: true,
		},
		{
			name:          "with extra whitespace",
			input:         "  /rcs   guidance with spaces   ",
			expectedText:  "guidance with spaces",
			expectedFound: true,
		},
		{
			name:          "no guidance",
			input:         "Just some regular text without guidance",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "empty string",
			input:         "",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "rcs without space",
			input:         "/rcsno space",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "rcs with only whitespace",
			input:         "/rcs   ",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "mixed case rcs",
			input:         "/RcS mixed case guidance",
			expectedText:  "mixed case guidance",
			expectedFound: true,
		},
		{
			name:          "text before rcs is ignored",
			input:         "Before\n/rcs important guidance\nAfter",
			expectedText:  "important guidance\nAfter",
			expectedFound: true,
		},
		{
			name:          "captures everything after first /rcs",
			input:         "/rcs first\nmore content\n/rcs second",
			expectedText:  "first\nmore content\n/rcs second",
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, found := ParseUserGuidance(tt.input)

			if found != tt.expectedFound {
				t.Errorf("Expected found=%v, got %v", tt.expectedFound, found)
			}

			if text != tt.expectedText {
				t.Errorf("Expected text '%s', got '%s'", tt.expectedText, text)
			}
		})
	}
}

func TestParseUserGuidanceEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedText  string
		expectedFound bool
	}{
		{
			name:          "guidance with special characters",
			input:         "/rcs Check the @deployment: it uses env vars!",
			expectedText:  "Check the @deployment: it uses env vars!",
			expectedFound: true,
		},
		{
			name:          "guidance with URLs",
			input:         "/rcs See https://example.com for details",
			expectedText:  "See https://example.com for details",
			expectedFound: true,
		},
		{
			name:          "captures text after /rcs including newlines",
			input:         "Line 1\nLine 2\n/rcs guidance here\nLine 3",
			expectedText:  "guidance here\nLine 3",
			expectedFound: true,
		},
		{
			name:          "multiple spaces after rcs",
			input:         "/rcs     multiple     spaces",
			expectedText:  "multiple     spaces",
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, found := ParseUserGuidance(tt.input)

			if found != tt.expectedFound {
				t.Errorf("Expected found=%v, got %v", tt.expectedFound, found)
			}

			if text != tt.expectedText {
				t.Errorf("Expected text '%s', got '%s'", tt.expectedText, text)
			}
		})
	}
}
