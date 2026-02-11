package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// TokenProvider defines an interface for obtaining an authentication token.
type TokenProvider interface {
	Token() (string, error)
}

// VertexEmbedder implements the Embedder interface using Google Cloud Vertex AI.
type VertexEmbedder struct {
	ProjectID     string
	Location      string
	Model         string
	TokenProvider TokenProvider
	BaseURL       string // Can be overridden for testing
	HTTPClient    *http.Client
}

// NewVertexEmbedder creates a new VertexEmbedder.
func NewVertexEmbedder(projectID, location string, tokenProvider TokenProvider) *VertexEmbedder {
	return &VertexEmbedder{
		ProjectID:     projectID,
		Location:      location,
		Model:         "text-embedding-004", // Default model
		TokenProvider: tokenProvider,
		BaseURL:       "", // Empty implies default construction
		HTTPClient:    &http.Client{},
	}
}

// VertexRequest represents the JSON payload for Vertex AI.
type vertexRequest struct {
	Instances []vertexInstance `json:"instances"`
}

type vertexInstance struct {
	Content string `json:"content"`
	Title   string `json:"title,omitempty"` // Optional: for retrieval tasks
}

// VertexResponse represents the JSON response from Vertex AI.
type vertexResponse struct {
	Predictions []vertexPrediction `json:"predictions"`
	Error       *vertexError       `json:"error,omitempty"`
}

type vertexPrediction struct {
	Embeddings vertexEmbeddingValues `json:"embeddings"`
}

type vertexEmbeddingValues struct {
	Values []float32 `json:"values"`
}

type vertexError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// EmbedBatch generates embeddings for a batch of texts.
func (v *VertexEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Construct URL
	// Default: https://us-central1-aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/LOCATION/publishers/google/models/MODEL:predict
	url := v.BaseURL
	if url == "" {
		url = fmt.Sprintf("https://%s-aiplatform.googleapis.com", v.Location)
	}
	endpoint := fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		url, v.ProjectID, v.Location, v.Model)

	// Construct Payload
	instances := make([]vertexInstance, len(texts))
	for i, t := range texts {
		instances[i] = vertexInstance{Content: t}
	}
	payload := vertexRequest{Instances: instances}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create Request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Headers
	req.Header.Set("Content-Type", "application/json")
	
	token, err := v.TokenProvider.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Execute
	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Vertex AI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vertex AI API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse Response
	var result vertexResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("vertex AI error %d: %s", result.Error.Code, result.Error.Message)
	}

	// Extract Embeddings
	output := make([][]float32, len(result.Predictions))
	for i, pred := range result.Predictions {
		output[i] = pred.Embeddings.Values
	}

	if len(output) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(output))
	}

	return output, nil
}
