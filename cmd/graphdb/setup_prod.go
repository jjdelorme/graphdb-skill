//go:build !test_mocks

package main

import (
	"graphdb/internal/embedding"
	"graphdb/internal/rpg"
	"os"
)

func setupEmbedder(project, location, token string) embedding.Embedder {
	if token == "" {
		token = os.Getenv("VERTEX_API_KEY") // Fallback
	}
	return embedding.NewVertexEmbedder(project, location, &SimpleTokenProvider{TokenString: token})
}

func setupSummarizer(project, location, token string) rpg.Summarizer {
	if token == "" {
		token = os.Getenv("VERTEX_API_KEY") // Fallback
	}
	return rpg.NewVertexSummarizer(project, location, &SimpleTokenProvider{TokenString: token})
}

func setupExtractor(project, location, token string) rpg.FeatureExtractor {
	if token == "" {
		token = os.Getenv("VERTEX_API_KEY")
	}
	return rpg.NewLLMFeatureExtractor(project, location, &SimpleTokenProvider{TokenString: token})
}
