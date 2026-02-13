//go:build !test_mocks

package main

import (
	"context"
	"graphdb/internal/embedding"
	"graphdb/internal/rpg"
	"log"
)

func setupEmbedder(project, location, modelName string) embedding.Embedder {
	ctx := context.Background()
	embedder, err := embedding.NewVertexEmbedder(ctx, project, location, modelName)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Embedder: %v", err)
	}
	return embedder
}

func setupSummarizer(project, location string) rpg.Summarizer {
	ctx := context.Background()
	summarizer, err := rpg.NewVertexSummarizer(ctx, project, location)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Summarizer: %v", err)
	}
	return summarizer
}

func setupExtractor(project, location string) rpg.FeatureExtractor {
	ctx := context.Background()
	extractor, err := rpg.NewLLMFeatureExtractor(ctx, project, location)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Feature Extractor: %v", err)
	}
	return extractor
}
