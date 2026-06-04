//go:build test_mocks

package main

import (
	"context"
	"graphdb/internal/config"
	"graphdb/internal/embedding"
	"graphdb/internal/loader"
	"graphdb/internal/query"
	"graphdb/internal/rpg"
	"log"
	"net/url"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func init() {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Mock Mode Initialized: Overriding enrichCmd")
		enrichCmd = handleMockEnrichFeatures
	}
}

func setupEmbedder(cfg config.Config) embedding.Embedder {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Embedder (test_mocks build)")
		return &MockEmbedder{}
	}

	ctx := context.Background()
	embedder, err := embedding.NewVertexEmbedder(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Embedder: %v", err)
	}
	return embedder
}

func setupSummarizer(cfg config.Config, appContext string) rpg.Summarizer {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Summarizer (test_mocks build)")
		return &MockSummarizer{}
	}

	ctx := context.Background()
	summarizer, err := rpg.NewVertexSummarizer(ctx, cfg, appContext)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Summarizer: %v", err)
	}
	return summarizer
}

func setupExtractor(cfg config.Config, appContext string) rpg.FeatureExtractor {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Feature Extractor (test_mocks build)")
		return &rpg.MockFeatureExtractor{}
	}

	ctx := context.Background()
	extractor, err := rpg.NewLLMFeatureExtractor(ctx, cfg, appContext)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Feature Extractor: %v", err)
	}
	return extractor
}

func setupProvider(cfg config.Config) (query.GraphProvider, error) {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Provider (test_mocks build)")
		return &MockProvider{}, nil
	}
	return query.NewNeo4jProvider(cfg)
}

func setupDriver(cfg config.Config) (neo4j.DriverWithContext, error) {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Driver (test_mocks build)")
		return &MockDriver{}, nil
	}
	return neo4j.NewDriverWithContext(cfg.Neo4jURI, neo4j.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, ""))
}

func setupLoader(ctx context.Context, cfg config.Config, driver neo4j.DriverWithContext) (loader.Loader, error) {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Loader (test_mocks build)")
		return &MockLoader{}, nil
	}
	return loader.NewNeo4jLoader(driver, cfg.Neo4jDatabase, cfg.GeminiEmbeddingDimensions), nil
}

// Mock implementation of neo4j.DriverWithContext
type MockDriver struct{}

func (m *MockDriver) ExecuteQueryBookmarkManager() neo4j.BookmarkManager { return nil }
func (m *MockDriver) Target() url.URL                                     { return url.URL{} }
func (m *MockDriver) NewSession(ctx context.Context, config neo4j.SessionConfig) neo4j.SessionWithContext {
	return nil
}
func (m *MockDriver) VerifyConnectivity(ctx context.Context) error { return nil }
func (m *MockDriver) VerifyAuthentication(ctx context.Context, auth *neo4j.AuthToken) error {
	return nil
}
func (m *MockDriver) Close(ctx context.Context) error { return nil }
func (m *MockDriver) IsEncrypted() bool               { return false }
func (m *MockDriver) GetServerInfo(ctx context.Context) (neo4j.ServerInfo, error) {
	return nil, nil
}
