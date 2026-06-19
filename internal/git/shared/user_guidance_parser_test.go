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
			input:         "/rcs note this is a guidance message",
			expectedText:  "this is a guidance message",
			expectedFound: true,
		},
		{
			name:          "case insensitive",
			input:         "/RCS NOTE This Is A Message",
			expectedText:  "This Is A Message",
			expectedFound: true,
		},
		{
			name:          "multiline guidance",
			input:         "/rcs note first line\nsecond line\nthird line",
			expectedText:  "first line\nsecond line\nthird line",
			expectedFound: true,
		},
		{
			name:          "with extra whitespace",
			input:         "  /rcs   note   guidance with spaces   ",
			expectedText:  "guidance with spaces",
			expectedFound: true,
		},
		{
			name:          "leading tabs should work",
			input:         "\t\t/rcs note guidance with tabs",
			expectedText:  "guidance with tabs",
			expectedFound: true,
		},
		{
			name:          "captures everything after subcommand",
			input:         "/rcs note first\nmore content\n/rcs note second",
			expectedText:  "first\nmore content\n/rcs note second",
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
			name:          "bare /rcs no longer matches",
			input:         "/rcs this is guidance",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "rcs without space",
			input:         "/rcsnote content",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "rcs note with only whitespace content",
			input:         "/rcs note   ",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "text before /rcs note should not match",
			input:         "Before\n/rcs note important guidance\nAfter",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "non-whitespace before /rcs note should not match",
			input:         "Some text /rcs note this should not match",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "inline /rcs note should not match",
			input:         "Please note: /rcs note this is important",
			expectedText:  "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, found := ParseUserGuidance(tt.input)

			if found != tt.expectedFound {
				t.Errorf("found = %v, want %v", found, tt.expectedFound)
			}

			if text != tt.expectedText {
				t.Errorf("text = %q, want %q", text, tt.expectedText)
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
			input:         "/rcs note Check the @deployment: it uses env vars!",
			expectedText:  "Check the @deployment: it uses env vars!",
			expectedFound: true,
		},
		{
			name:          "guidance with URLs",
			input:         "/rcs note See https://example.com for details",
			expectedText:  "See https://example.com for details",
			expectedFound: true,
		},
		{
			name:          "text before /rcs note should not match",
			input:         "Line 1\nLine 2\n/rcs note guidance here\nLine 3",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "multiple spaces after note subcommand",
			input:         "/rcs note     multiple     spaces",
			expectedText:  "multiple     spaces",
			expectedFound: true,
		},
		{
			name:          "non-whitespace before /rcs note should not match",
			input:         "Some text /rcs note this should not match",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "inline /rcs note should not match",
			input:         "Please note: /rcs note this is important",
			expectedText:  "",
			expectedFound: false,
		},
		{
			name:          "/rcs override does not match note pattern",
			input:         "/rcs override proceeding with justification",
			expectedText:  "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, found := ParseUserGuidance(tt.input)

			if found != tt.expectedFound {
				t.Errorf("found = %v, want %v", found, tt.expectedFound)
			}

			if text != tt.expectedText {
				t.Errorf("text = %q, want %q", text, tt.expectedText)
			}
		})
	}
}
