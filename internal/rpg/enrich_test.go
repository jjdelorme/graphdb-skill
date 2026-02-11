package rpg

import (
	"graphdb/internal/graph"
	"testing"
)

type MockSummarizer struct{}

func (m *MockSummarizer) Summarize(snippets []string) (string, string, error) {
	return "User Login", "Handles authentication verification", nil
}

func TestEnricher_Enrich(t *testing.T) {
	enricher := &Enricher{
		Client: &MockSummarizer{},
	}

	feature := &Feature{
		ID:   "feat-temp",
		Name: "Cluster-001",
	}

	functions := []graph.Node{
		{Properties: map[string]interface{}{"content": "func login() { ... }"}},
		{Properties: map[string]interface{}{"content": "func verify() { ... }"}},
	}

	err := enricher.Enrich(feature, functions)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if feature.Name != "User Login" {
		t.Errorf("Expected name 'User Login', got '%s'", feature.Name)
	}
	if feature.Description != "Handles authentication verification" {
		t.Errorf("Expected description to match mock, got '%s'", feature.Description)
	}
}
