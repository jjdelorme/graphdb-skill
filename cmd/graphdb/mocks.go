//go:build test_mocks

package main

import (
	"fmt"
)

// MockEmbedder for testing/dry-run
type MockEmbedder struct{}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = make([]float32, 768) // Dummy 768-dim vector
	}
	return res, nil
}

// MockSummarizer for placeholder RPG
type MockSummarizer struct{}

func (s *MockSummarizer) Summarize(snippets []string) (string, string, error) {
	return "Mock Feature", "Automatically generated description based on " + fmt.Sprintf("%d", len(snippets)) + " snippets.", nil
}
