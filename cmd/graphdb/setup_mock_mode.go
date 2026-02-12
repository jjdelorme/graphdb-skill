//go:build test_mocks

package main

import (
	"context"
	"graphdb/internal/embedding"
	"graphdb/internal/rpg"
	"log"
	"os"
)

func setupEmbedder(project, location string) embedding.Embedder {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Embedder (test_mocks build)")
		return &MockEmbedder{}
	}

	ctx := context.Background()
	embedder, err := embedding.NewVertexEmbedder(ctx, project, location)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Embedder: %v", err)
	}
	return embedder
}

func setupSummarizer(project, location string) rpg.Summarizer {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Summarizer (test_mocks build)")
		return &MockSummarizer{}
	}

	ctx := context.Background()
	summarizer, err := rpg.NewVertexSummarizer(ctx, project, location)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Summarizer: %v", err)
	}
	return summarizer
}

func setupExtractor(project, location string) rpg.FeatureExtractor {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Feature Extractor (test_mocks build)")
		return &rpg.MockFeatureExtractor{}
	}

	ctx := context.Background()
	extractor, err := rpg.NewLLMFeatureExtractor(ctx, project, location)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Feature Extractor: %v", err)
	}
	return extractor
}
