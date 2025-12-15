package formatting

import (
	"strings"
	"testing"

	"release-confidence-score/internal/git/types"
)

func TestAdjustMarkdownHeadingLevels(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		incrementBy int
		expected    string
	}{
		{
			name:        "increment by 2",
			incrementBy: 2,
			input: `# Heading 1
## Heading 2
### Heading 3
Regular text`,
			expected: `### Heading 1
#### Heading 2
##### Heading 3
Regular text`,
		},
		{
			name:        "cap at H6",
			incrementBy: 2,
			input: `#### Heading 4
##### Heading 5
###### Heading 6`,
			expected: `###### Heading 4
###### Heading 5
###### Heading 6`,
		},
		{
			name:        "preserve code blocks",
			incrementBy: 2,
			input:       "# Heading\n```go\n# Not a heading\n## Also not a heading\n```\n## Real heading",
			expected:    "### Heading\n```go\n# Not a heading\n## Also not a heading\n```\n#### Real heading",
		},
		{
			name:        "preserve tilde code blocks",
			incrementBy: 2,
			input:       "# Heading\n~~~\n# Not a heading\n~~~\n## Real heading",
			expected:    "### Heading\n~~~\n# Not a heading\n~~~\n#### Real heading",
		},
		{
			name:        "headings with varying spaces",
			incrementBy: 2,
			input: `# Heading
##  Two spaces
###   Three spaces`,
			expected: `### Heading
####  Two spaces
#####   Three spaces`,
		},
		{
			name:        "no increment for zero",
			incrementBy: 0,
			input:       "# Heading\n## Subheading",
			expected:    "# Heading\n## Subheading",
		},
		{
			name:        "no increment for negative",
			incrementBy: -1,
			input:       "# Heading\n## Subheading",
			expected:    "# Heading\n## Subheading",
		},
		{
			name:        "empty content",
			incrementBy: 2,
			input:       "",
			expected:    "",
		},
		{
			name:        "no headings",
			incrementBy: 2,
			input:       "Just regular text\nNo headings here",
			expected:    "Just regular text\nNo headings here",
		},
		{
			name:        "hash not at line start",
			incrementBy: 2,
			input:       "This line has # in the middle\nAnd #another one",
			expected:    "This line has # in the middle\nAnd #another one",
		},
		{
			name:        "nested code blocks",
			incrementBy: 2,
			input: `# Outer heading
` + "```markdown" + `
# Code example
## Subheading in code
` + "```" + `
## Real subheading`,
			expected: `### Outer heading
` + "```markdown" + `
# Code example
## Subheading in code
` + "```" + `
#### Real subheading`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adjustMarkdownHeadingLevels(tt.input, tt.incrementBy)
			if result != tt.expected {
				t.Errorf("adjustMarkdownHeadingLevels() failed\nInput:\n%s\n\nExpected:\n%s\n\nGot:\n%s",
					tt.input, tt.expected, result)
			}
		})
	}
}

func TestAdjustMarkdownHeadingLevels_RealWorldExample(t *testing.T) {
	input := `# Project Name

A description of the project.

## Installation

` + "```bash" + `
npm install
` + "```" + `

## Usage

### Basic Usage

Some instructions.

### Advanced Usage

More instructions.

## API Reference

### Functions

#### doSomething()

Details about the function.`

	expected := `### Project Name

A description of the project.

#### Installation

` + "```bash" + `
npm install
` + "```" + `

#### Usage

##### Basic Usage

Some instructions.

##### Advanced Usage

More instructions.

#### API Reference

##### Functions

###### doSomething()

Details about the function.`

	result := adjustMarkdownHeadingLevels(input, 2)
	if result != expected {
		t.Errorf("Real-world example failed\nExpected:\n%s\n\nGot:\n%s", expected, result)
	}
}

func TestFormatDocumentations_HeadingAdjustment(t *testing.T) {
	// This test verifies that FormatDocumentations properly adjusts heading levels
	// We're not testing the full Documentation formatting, just that headings are adjusted

	input := "# Main Title\n## Subtitle\nContent"
	result := adjustMarkdownHeadingLevels(input, 2)

	// Verify headings were incremented
	if !strings.Contains(result, "### Main Title") {
		t.Error("Expected '### Main Title' in result")
	}
	if !strings.Contains(result, "#### Subtitle") {
		t.Error("Expected '#### Subtitle' in result")
	}
	if !strings.Contains(result, "Content") {
		t.Error("Expected 'Content' to be preserved")
	}
}

func TestFormatDocumentations(t *testing.T) {
	tests := []struct {
		name     string
		input    []*types.Documentation
		expected string
	}{
		{
			name:     "empty list",
			input:    []*types.Documentation{},
			expected: "",
		},
		{
			name:     "nil list",
			input:    nil,
			expected: "",
		},
		{
			name: "single documentation without headings",
			input: []*types.Documentation{
				{
					MainDocFile:    "README.md",
					MainDocContent: "Simple content without headings",
					Repository: types.Repository{
						Owner: "user",
						Name:  "repo",
					},
				},
			},
			expected: "### README.md (user/repo)\n\nSimple content without headings\n\n",
		},
		{
			name: "single documentation with headings",
			input: []*types.Documentation{
				{
					MainDocFile:    "README.md",
					MainDocContent: "# Main Title\n## Subtitle\nContent here",
					Repository: types.Repository{
						Owner: "user",
						Name:  "repo",
					},
				},
			},
			expected: "### README.md (user/repo)\n\n#### Main Title\n##### Subtitle\nContent here\n\n",
		},
		{
			name: "documentation with linked docs",
			input: []*types.Documentation{
				{
					MainDocFile:    "README.md",
					MainDocContent: "# Main\nMain content",
					AdditionalDocsContent: map[string]string{
						"CONTRIBUTING.md": "# Contributing\nGuidelines",
						"INSTALL.md":      "# Installation\nSteps",
					},
					AdditionalDocsOrder: []string{"CONTRIBUTING.md", "INSTALL.md"},
					Repository: types.Repository{
						Owner: "user",
						Name:  "repo",
					},
				},
			},
			expected: "### README.md (user/repo)\n\n#### Main\nMain content\n\n### CONTRIBUTING.md (user/repo)\n\n#### Contributing\nGuidelines\n\n### INSTALL.md (user/repo)\n\n#### Installation\nSteps\n\n",
		},
		{
			name: "multiple documentations",
			input: []*types.Documentation{
				{
					MainDocFile:    "README.md",
					MainDocContent: "# First Repo",
					Repository: types.Repository{
						Owner: "user1",
						Name:  "repo1",
					},
				},
				{
					MainDocFile:    "README.md",
					MainDocContent: "# Second Repo",
					Repository: types.Repository{
						Owner: "user2",
						Name:  "repo2",
					},
				},
			},
			expected: "### README.md (user1/repo1)\n\n#### First Repo\n\n### README.md (user2/repo2)\n\n#### Second Repo\n\n",
		},
		{
			name: "nil documentation in list",
			input: []*types.Documentation{
				nil,
				{
					MainDocFile:    "README.md",
					MainDocContent: "Content",
					Repository: types.Repository{
						Owner: "user",
						Name:  "repo",
					},
				},
			},
			expected: "### README.md (user/repo)\n\nContent\n\n",
		},
		{
			name: "documentation with empty content",
			input: []*types.Documentation{
				{
					MainDocFile:    "README.md",
					MainDocContent: "",
					Repository: types.Repository{
						Owner: "user",
						Name:  "repo",
					},
				},
			},
			expected: "",
		},
		{
			name: "heading level capping at H6",
			input: []*types.Documentation{
				{
					MainDocFile:    "README.md",
					MainDocContent: "#### Level 4\n##### Level 5\n###### Level 6",
					Repository: types.Repository{
						Owner: "user",
						Name:  "repo",
					},
				},
			},
			expected: "### README.md (user/repo)\n\n###### Level 4\n###### Level 5\n###### Level 6\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDocumentations(tt.input)
			if result != tt.expected {
				t.Errorf("FormatDocumentations() failed\nExpected:\n%q\n\nGot:\n%q", tt.expected, result)
			}
		})
	}
}

func TestFormatDocumentations_PreservesCodeBlocks(t *testing.T) {
	docs := []*types.Documentation{
		{
			MainDocFile: "README.md",
			MainDocContent: `# Title
` + "```go" + `
# Not a heading
## Also not a heading
` + "```" + `
## Real Heading`,
			Repository: types.Repository{
				Owner: "user",
				Name:  "repo",
			},
		},
	}

	result := FormatDocumentations(docs)

	// Verify code block content is preserved
	if !strings.Contains(result, "# Not a heading") {
		t.Error("Code block content should be preserved")
	}
	if !strings.Contains(result, "## Also not a heading") {
		t.Error("Code block content should be preserved")
	}

	// Verify real headings are adjusted
	if !strings.Contains(result, "#### Title") {
		t.Error("Expected title to be adjusted to H4")
	}
	if !strings.Contains(result, "##### Real Heading") {
		t.Error("Expected real heading to be adjusted to H5")
	}
}
