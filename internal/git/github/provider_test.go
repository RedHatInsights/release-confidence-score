package github

import (
	"testing"

	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/github/graphql"
	"release-confidence-score/internal/git/github/rest"
)

func TestNewProvider_ReturnsRESTByDefault(t *testing.T) {
	cfg := &config.Config{
		GitHubToken:      "test-token",
		GitHubUseGraphQL: false,
	}

	provider := NewProvider(cfg)

	// Should return REST fetcher by default
	_, isREST := provider.(*rest.Fetcher)
	if !isREST {
		t.Error("NewProvider should return REST Fetcher when GitHubUseGraphQL is false")
	}
}

func TestNewProvider_ReturnsGraphQLWhenEnabled(t *testing.T) {
	cfg := &config.Config{
		GitHubToken:      "test-token",
		GitHubUseGraphQL: true,
	}

	provider := NewProvider(cfg)

	// Should return GraphQL fetcher when enabled
	_, isGraphQL := provider.(*graphql.Fetcher)
	if !isGraphQL {
		t.Error("NewProvider should return GraphQL Fetcher when GitHubUseGraphQL is true")
	}
}
