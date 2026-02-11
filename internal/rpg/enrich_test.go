package rpg

import (
	"graphdb/internal/graph"
	"testing"
)

type MockSummarizer struct{}

func (m *MockSummarizer) Summarize(snippets []string) (string, string, error) {
	return "User Login", "Handles authentication verification", nil
}

type MockEmbedder struct{}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = make([]float32, 768)
		res[i][0] = 0.42 // Sentinel value for testing
	}
	return res, nil
}

func TestEnricher_Enrich(t *testing.T) {
	enricher := &Enricher{
		Client:   &MockSummarizer{},
		Embedder: &MockEmbedder{},
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
	if feature.Embedding == nil {
		t.Fatal("Expected Embedding to be non-nil after Enrich")
	}
	if len(feature.Embedding) != 768 {
		t.Errorf("Expected 768-dim embedding, got %d", len(feature.Embedding))
	}
	if feature.Embedding[0] != 0.42 {
		t.Errorf("Expected sentinel value 0.42 in embedding[0], got %f", feature.Embedding[0])
	}
}

func TestEnricher_Enrich_NilEmbedder(t *testing.T) {
	enricher := &Enricher{
		Client: &MockSummarizer{},
		// Embedder is nil -- should still work, just no embedding
	}

	feature := &Feature{
		ID:   "feat-temp",
		Name: "Cluster-001",
	}

	functions := []graph.Node{
		{Properties: map[string]interface{}{"content": "func login() { ... }"}},
	}

	err := enricher.Enrich(feature, functions)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if feature.Name != "User Login" {
		t.Errorf("Expected name 'User Login', got '%s'", feature.Name)
	}
	if feature.Embedding != nil {
		t.Errorf("Expected nil embedding when embedder is nil, got %v", feature.Embedding)
	}
}
