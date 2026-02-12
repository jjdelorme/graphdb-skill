package embedding

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/genai"
)

// VertexEmbedder implements the Embedder interface using Google Cloud Vertex AI via the GenAI SDK.
type VertexEmbedder struct {
	Client *genai.Client
	Model  string
}

// NewVertexEmbedder creates a new VertexEmbedder.
func NewVertexEmbedder(ctx context.Context, projectID, location string) (*VertexEmbedder, error) {
	// Initialize the client with Vertex AI backend configuration
	// This automatically uses Application Default Credentials (ADC)
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &VertexEmbedder{
		Client: client,
		Model:  "text-embedding-004",
	}, nil
}

// EmbedBatch generates embeddings for a batch of texts.
func (v *VertexEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	ctx := context.Background()
	var batch []*genai.Content
	for _, t := range texts {
		// genai.Text returns []*Content (slice of content parts), usually one for simple text.
		batch = append(batch, genai.Text(t)...)
	}

	resp, err := v.Client.Models.EmbedContent(ctx, v.Model, batch, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to embed content batch: %w", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from embedding service")
	}

	if len(resp.Embeddings) != len(texts) {
		log.Printf("Warning: requested %d embeddings, got %d", len(texts), len(resp.Embeddings))
		// We might still return what we have, or error. 
		// If partial, it's safer to error as alignment is lost.
		if len(resp.Embeddings) < len(texts) {
			return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(texts), len(resp.Embeddings))
		}
	}

	allEmbeddings := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		if emb != nil {
			allEmbeddings[i] = emb.Values
		}
	}

	return allEmbeddings, nil
}
