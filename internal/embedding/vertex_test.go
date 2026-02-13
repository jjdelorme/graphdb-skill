package embedding

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/genai"
)

type mockModelClient struct {
	embedFunc func(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error)
}

func (m *mockModelClient) EmbedContent(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
	return m.embedFunc(ctx, model, contents, config)
}

func TestVertexEmbedder_EmbedBatch_Chunking(t *testing.T) {
	callCount := 0
	mock := &mockModelClient{
		embedFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
			callCount++
			// Return embeddings matching input size
			embeddings := make([]*genai.ContentEmbedding, len(contents))
			for i := range contents {
				// Use callCount and index to verify order and batching
				embeddings[i] = &genai.ContentEmbedding{Values: []float32{float32(callCount), float32(i)}}
			}
			return &genai.EmbedContentResponse{Embeddings: embeddings}, nil
		},
	}

	embedder := &VertexEmbedder{
		Client: mock,
		Model:  "test-model",
	}

	// 250 items should result in 3 batches (100, 100, 50)
	texts := make([]string, 250)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	res, err := embedder.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls to API, got %d", callCount)
	}

	if len(res) != 250 {
		t.Errorf("Expected 250 embeddings, got %d", len(res))
	}
    
	// Verify alignment
	// First item of first batch
	if res[0][0] != 1 || res[0][1] != 0 {
		t.Errorf("Alignment check failed at index 0: got %v", res[0])
	}
	// First item of second batch (index 100)
	if res[100][0] != 2 || res[100][1] != 0 {
		t.Errorf("Alignment check failed at index 100: got %v", res[100])
	}
	// Last item of second batch (index 199)
	if res[199][0] != 2 || res[199][1] != 99 {
		t.Errorf("Alignment check failed at index 199: got %v", res[199])
	}
	// First item of third batch (index 200)
	if res[200][0] != 3 || res[200][1] != 0 {
		t.Errorf("Alignment check failed at index 200: got %v", res[200])
	}
}

func TestVertexEmbedder_EmbedBatch_MismatchError(t *testing.T) {
	mock := &mockModelClient{
		embedFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
			// Return fewer embeddings than requested
			return &genai.EmbedContentResponse{Embeddings: make([]*genai.ContentEmbedding, len(contents)-1)}, nil
		},
	}

	embedder := &VertexEmbedder{
		Client: mock,
		Model:  "test-model",
	}

	texts := []string{"a", "b", "c"}
	_, err := embedder.EmbedBatch(texts)
	if err == nil {
		t.Fatal("Expected error due to mismatch, got nil")
	}
	if err.Error() == "" {
		t.Fatal("Expected error message, got empty string")
	}
}

func TestVertexEmbedder_EmbedBatch_Empty(t *testing.T) {
	embedder := &VertexEmbedder{
		Client: &mockModelClient{},
	}

	res, err := embedder.EmbedBatch(nil)
	if err != nil {
		t.Errorf("Expected nil error for nil input, got %v", err)
	}
	if res != nil {
		t.Errorf("Expected nil result for nil input, got %v", res)
	}
}
