package embedding

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// ModelClient defines the subset of genai.Client methods we use, allowing for testing.
type ModelClient interface {
	EmbedContent(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error)
}

// VertexEmbedder implements the Embedder interface using Google Cloud Vertex AI via the GenAI SDK.
type VertexEmbedder struct {
	Client ModelClient
	Model  string
}

// NewVertexEmbedder creates a new VertexEmbedder.
func NewVertexEmbedder(ctx context.Context, projectID, location, modelName string) (*VertexEmbedder, error) {
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
		Client: client.Models,
		Model:  modelName,
	}, nil
}

// EmbedBatch generates embeddings for a batch of texts.
func (v *VertexEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	ctx := context.Background()

	// Vertex AI typically has a limit of 250 items per batch request.
	// We use a safe batch size of 100 to stay well within limits.
	const batchSize = 100

	total := len(texts)
	allEmbeddings := make([][]float32, 0, total)

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		chunkTexts := texts[i:end]
		var batch []*genai.Content
		for _, t := range chunkTexts {
			if t == "" {
				t = " "
			}
			batch = append(batch, genai.NewContentFromText(t, genai.RoleUser))
		}

		config := &genai.EmbedContentConfig{
			TaskType:     "RETRIEVAL_DOCUMENT",
			AutoTruncate: true,
		}

		resp, err := v.Client.EmbedContent(ctx, v.Model, batch, config)
		if err != nil {
			return nil, fmt.Errorf("failed to embed content batch (chunk %d-%d): %w", i, end, err)
		}

		if resp == nil {
			return nil, fmt.Errorf("empty response from embedding service for chunk %d-%d", i, end)
		}

		if len(resp.Embeddings) != len(chunkTexts) {
			return nil, fmt.Errorf("embedding count mismatch in chunk %d-%d: expected %d, got %d", i, end, len(chunkTexts), len(resp.Embeddings))
		}

		for _, emb := range resp.Embeddings {
			if emb != nil {
				allEmbeddings = append(allEmbeddings, emb.Values)
			} else {
				allEmbeddings = append(allEmbeddings, []float32{})
			}
		}
	}

	return allEmbeddings, nil
}
