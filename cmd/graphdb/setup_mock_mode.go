//go:build test_mocks

package main

import (
	"graphdb/internal/embedding"
	"graphdb/internal/rpg"
	"log"
	"os"
)

func setupEmbedder(project, location, token string) embedding.Embedder {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Embedder (test_mocks build)")
		return &MockEmbedder{}
	}

	if token == "" {
		token = os.Getenv("VERTEX_API_KEY") // Fallback
	}
	return embedding.NewVertexEmbedder(project, location, &SimpleTokenProvider{TokenString: token})
}

func setupSummarizer(project, location, token string) rpg.Summarizer {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Summarizer (test_mocks build)")
		return &MockSummarizer{}
	}

	if token == "" {
		token = os.Getenv("VERTEX_API_KEY") // Fallback
	}
	return rpg.NewVertexSummarizer(project, location, &SimpleTokenProvider{TokenString: token})
}

func setupExtractor(project, location, token string) rpg.FeatureExtractor {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Feature Extractor (test_mocks build)")
		return &rpg.MockFeatureExtractor{}
	}

	if token == "" {
		token = os.Getenv("VERTEX_API_KEY")
	}
	return rpg.NewLLMFeatureExtractor(project, location, &SimpleTokenProvider{TokenString: token})
}
