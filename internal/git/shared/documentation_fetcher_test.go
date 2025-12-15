package shared

import (
	"context"
	"errors"
	"testing"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/types"
)

// mockDocumentationSource implements types.DocumentationSource for testing
type mockDocumentationSource struct {
	defaultBranch    string
	defaultBranchErr error
	files            map[string]string // path -> content
	fileErrors       map[string]error  // path -> error
}

func (m *mockDocumentationSource) GetDefaultBranch(ctx context.Context) (string, error) {
	if m.defaultBranchErr != nil {
		return "", m.defaultBranchErr
	}
	return m.defaultBranch, nil
}

func (m *mockDocumentationSource) FetchFileContent(ctx context.Context, path, ref string) (string, error) {
	if err, ok := m.fileErrors[path]; ok {
		return "", err
	}
	if content, ok := m.files[path]; ok {
		return content, nil
	}
	return "", errors.New("file not found")
}

func TestNewDocumentationFetcher(t *testing.T) {
	source := &mockDocumentationSource{}
	repo := types.Repository{Owner: "test", Name: "repo"}
	cfg := &config.Config{}

	fetcher := NewDocumentationFetcher(source, repo, cfg)

	if fetcher.source != source {
		t.Error("source not set correctly")
	}
	if fetcher.baseRepository != repo {
		t.Error("baseRepository not set correctly")
	}
	if fetcher.config != cfg {
		t.Error("config not set correctly")
	}
}

func TestFetchAllDocs_NoMainDoc(t *testing.T) {
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files:         map[string]string{},
	}
	repo := types.Repository{Owner: "test", Name: "repo", URL: "https://github.com/test/repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	docs, err := fetcher.FetchAllDocs(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if docs.MainDocContent != "" {
		t.Error("expected empty main doc content")
	}
	if docs.Repository.DefaultBranch != "main" {
		t.Errorf("expected default branch 'main', got: %s", docs.Repository.DefaultBranch)
	}
}

func TestFetchAllDocs_WithMainDocNoAdditional(t *testing.T) {
	mainDocContent := "# Main Documentation\n\nThis is the main doc."
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files: map[string]string{
			mainDocFilename: mainDocContent,
		},
	}
	repo := types.Repository{Owner: "test", Name: "repo", URL: "https://github.com/test/repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	docs, err := fetcher.FetchAllDocs(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if docs.MainDocContent != mainDocContent {
		t.Errorf("expected main doc content, got: %s", docs.MainDocContent)
	}
	if len(docs.AdditionalDocsContent) != 0 {
		t.Error("expected no additional docs")
	}
}

func TestFetchAllDocs_WithAdditionalDocs(t *testing.T) {
	mainDocContent := `# Main Documentation

## Additional Documentation
- [Guide](docs/guide.md)
- [API](docs/api.md)
`
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files: map[string]string{
			mainDocFilename: mainDocContent,
			"docs/guide.md": "Guide content",
			"docs/api.md":   "API content",
		},
	}
	repo := types.Repository{Owner: "test", Name: "repo", URL: "https://github.com/test/repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	docs, err := fetcher.FetchAllDocs(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(docs.AdditionalDocsContent) != 2 {
		t.Errorf("expected 2 additional docs, got: %d", len(docs.AdditionalDocsContent))
	}
	if docs.AdditionalDocsContent["Guide"] != "Guide content" {
		t.Error("Guide content not fetched correctly")
	}
	if docs.AdditionalDocsContent["API"] != "API content" {
		t.Error("API content not fetched correctly")
	}
	if len(docs.AdditionalDocsOrder) != 2 {
		t.Errorf("expected 2 items in order, got: %d", len(docs.AdditionalDocsOrder))
	}
	if docs.AdditionalDocsOrder[0] != "Guide" || docs.AdditionalDocsOrder[1] != "API" {
		t.Errorf("expected order [Guide, API], got: %v", docs.AdditionalDocsOrder)
	}
}

func TestFetchAllDocs_WithFailedAdditionalDocs(t *testing.T) {
	mainDocContent := `# Main Documentation

## Additional Documentation
- [Guide](docs/guide.md)
- [Missing](docs/missing.md)
`
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files: map[string]string{
			mainDocFilename: mainDocContent,
			"docs/guide.md": "Guide content",
		},
		fileErrors: map[string]error{
			"docs/missing.md": errors.New("file not found"),
		},
	}
	repo := types.Repository{Owner: "test", Name: "repo", URL: "https://github.com/test/repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	docs, err := fetcher.FetchAllDocs(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(docs.AdditionalDocsContent) != 1 {
		t.Errorf("expected 1 successful additional doc, got: %d", len(docs.AdditionalDocsContent))
	}
	if len(docs.FailedAdditionalDocs) != 1 {
		t.Errorf("expected 1 failed additional doc, got: %d", len(docs.FailedAdditionalDocs))
	}
	if _, ok := docs.FailedAdditionalDocs["Missing"]; !ok {
		t.Error("expected 'Missing' in failed docs")
	}
	// Order should still include both
	if len(docs.AdditionalDocsOrder) != 2 {
		t.Errorf("expected 2 items in order, got: %d", len(docs.AdditionalDocsOrder))
	}
}

func TestFetchAllDocs_DefaultBranchError(t *testing.T) {
	source := &mockDocumentationSource{
		defaultBranchErr: errors.New("API error"),
	}
	repo := types.Repository{Owner: "test", Name: "repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	_, err := fetcher.FetchAllDocs(context.Background())

	if err == nil {
		t.Fatal("expected error when getting default branch fails")
	}
}

func TestExtractAdditionalDocPaths_NoSection(t *testing.T) {
	content := "# Main Documentation\n\nNo additional docs section."

	paths, order := extractAdditionalDocPaths(content)

	if paths != nil {
		t.Error("expected nil paths when no section exists")
	}
	if order != nil {
		t.Error("expected nil order when no section exists")
	}
}

func TestExtractAdditionalDocPaths_MarkdownLinks(t *testing.T) {
	content := `# Main Documentation

## Additional Documentation
- [Guide](docs/guide.md)
- [API Documentation](docs/api.md)
`

	paths, order := extractAdditionalDocPaths(content)

	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got: %d", len(paths))
	}
	if paths["Guide"] != "docs/guide.md" {
		t.Errorf("expected 'Guide' -> 'docs/guide.md', got: %s", paths["Guide"])
	}
	if paths["API Documentation"] != "docs/api.md" {
		t.Errorf("expected 'API Documentation' -> 'docs/api.md', got: %s", paths["API Documentation"])
	}
	if len(order) != 2 {
		t.Errorf("expected 2 items in order, got: %d", len(order))
	}
	if order[0] != "Guide" || order[1] != "API Documentation" {
		t.Errorf("expected order [Guide, API Documentation], got: %v", order)
	}
}

func TestExtractAdditionalDocPaths_PlainURLs(t *testing.T) {
	content := `# Main Documentation

## Additional Documentation
https://example.com/doc1.md
https://example.com/doc2.md
`

	paths, order := extractAdditionalDocPaths(content)

	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got: %d", len(paths))
	}
	url1 := "https://example.com/doc1.md"
	url2 := "https://example.com/doc2.md"
	if paths[url1] != url1 {
		t.Error("URL not mapped to itself")
	}
	if paths[url2] != url2 {
		t.Error("URL not mapped to itself")
	}
	if order[0] != url1 || order[1] != url2 {
		t.Errorf("expected order [%s, %s], got: %v", url1, url2, order)
	}
}

func TestExtractAdditionalDocPaths_MixedLinksAndURLs(t *testing.T) {
	content := `# Main Documentation

## Additional Documentation
- [Guide](docs/guide.md)
https://example.com/external.md
- [API](docs/api.md)
`

	paths, order := extractAdditionalDocPaths(content)

	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got: %d", len(paths))
	}
	// Order should be: markdown links first, then plain URLs
	expectedOrder := []string{"Guide", "API", "https://example.com/external.md"}
	if len(order) != 3 {
		t.Fatalf("expected 3 items in order, got: %d", len(order))
	}
	for i, expected := range expectedOrder {
		if order[i] != expected {
			t.Errorf("order[%d]: expected %s, got: %s", i, expected, order[i])
		}
	}
}

func TestExtractAdditionalDocPaths_DuplicatePaths(t *testing.T) {
	content := `# Main Documentation

## Additional Documentation
- [Doc1](docs/guide.md)
- [Doc2](docs/guide.md)
`

	paths, order := extractAdditionalDocPaths(content)

	// Both display names are preserved (different keys, same value)
	if len(paths) != 2 {
		t.Errorf("expected 2 paths (different display names), got: %d", len(paths))
	}
	if len(order) != 2 {
		t.Errorf("expected 2 items in order, got: %d", len(order))
	}
	if paths["Doc1"] != "docs/guide.md" || paths["Doc2"] != "docs/guide.md" {
		t.Error("both display names should map to same path")
	}
}

func TestExtractAdditionalDocSection_SectionExists(t *testing.T) {
	content := `# Main Documentation

Some content.

## Additional Documentation
Link 1
Link 2

## Another Section
Other content
`

	section := extractAdditionalDocSection(content)

	expected := "Link 1\nLink 2"
	if section != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, section)
	}
}

func TestExtractAdditionalDocSection_NoSection(t *testing.T) {
	content := "# Main Documentation\n\nNo additional docs section."

	section := extractAdditionalDocSection(content)

	if section != "" {
		t.Errorf("expected empty string, got: %s", section)
	}
}

func TestExtractAdditionalDocSection_SectionAtEnd(t *testing.T) {
	content := `# Main Documentation

## Additional Documentation
Link 1
Link 2`

	section := extractAdditionalDocSection(content)

	expected := "Link 1\nLink 2"
	if section != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, section)
	}
}

func TestFetchAdditionalDocs_AllSuccessful(t *testing.T) {
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files: map[string]string{
			"docs/guide.md": "Guide content",
			"docs/api.md":   "API content",
		},
	}
	repo := types.Repository{Owner: "test", Name: "repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	paths := map[string]string{
		"Guide": "docs/guide.md",
		"API":   "docs/api.md",
	}
	order := []string{"Guide", "API"}

	docs, failed := fetcher.fetchAdditionalDocs(context.Background(), "main", paths, order)

	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got: %d", len(docs))
	}
	if len(failed) != 0 {
		t.Errorf("expected 0 failed, got: %d", len(failed))
	}
	if docs["Guide"] != "Guide content" {
		t.Error("Guide content incorrect")
	}
	if docs["API"] != "API content" {
		t.Error("API content incorrect")
	}
}

func TestFetchAdditionalDocs_WithFailures(t *testing.T) {
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files: map[string]string{
			"docs/guide.md": "Guide content",
		},
		fileErrors: map[string]error{
			"docs/missing.md": errors.New("not found"),
		},
	}
	repo := types.Repository{Owner: "test", Name: "repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	paths := map[string]string{
		"Guide":   "docs/guide.md",
		"Missing": "docs/missing.md",
	}
	order := []string{"Guide", "Missing"}

	docs, failed := fetcher.fetchAdditionalDocs(context.Background(), "main", paths, order)

	if len(docs) != 1 {
		t.Errorf("expected 1 successful doc, got: %d", len(docs))
	}
	if len(failed) != 1 {
		t.Errorf("expected 1 failed doc, got: %d", len(failed))
	}
	if _, ok := failed["Missing"]; !ok {
		t.Error("expected 'Missing' in failed docs")
	}
}

func TestFetchAdditionalDocContent_RepositoryFile(t *testing.T) {
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files: map[string]string{
			"docs/guide.md": "Guide content",
		},
	}
	repo := types.Repository{Owner: "test", Name: "repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	content, err := fetcher.fetchAdditionalDocContent(context.Background(), "main", "docs/guide.md")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if content != "Guide content" {
		t.Errorf("expected 'Guide content', got: %s", content)
	}
}

func TestFetchAdditionalDocContent_RepositoryFileNotFound(t *testing.T) {
	source := &mockDocumentationSource{
		defaultBranch: "main",
		files:         map[string]string{},
	}
	repo := types.Repository{Owner: "test", Name: "repo"}
	cfg := &config.Config{}
	fetcher := NewDocumentationFetcher(source, repo, cfg)

	_, err := fetcher.fetchAdditionalDocContent(context.Background(), "main", "docs/missing.md")

	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
