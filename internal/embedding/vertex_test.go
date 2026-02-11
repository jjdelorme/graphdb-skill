package embedding_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"graphdb/internal/embedding"
)

// MockTokenSource implements a simple token source for testing
type MockTokenSource struct{}

func (m *MockTokenSource) Token() (string, error) {
	return "fake-token", nil
}

func TestVertexEmbedder_EmbedBatch(t *testing.T) {
	// 1. Setup a fake Vertex AI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL format: https://us-central1-aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/LOCATION/publishers/google/models/text-embedding-004:predict
		if r.URL.Path != "/v1/projects/my-project/locations/us-central1/publishers/google/models/text-embedding-004:predict" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// Verify Auth Header
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			t.Errorf("Missing or invalid Authorization header")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Return a fake response
		responseJSON := `{
			"predictions": [
				{
					"embeddings": {
						"values": [0.1, 0.2, 0.3]
					}
				},
				{
					"embeddings": {
						"values": [0.4, 0.5, 0.6]
					}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseJSON))
	}))
	defer server.Close()

	// 2. Initialize the Embedder with the fake server URL
	// We need a way to override the Base URL. 
	// The NewVertexEmbedder should probably accept functional options or a config struct.
	// For now, let's assume a constructor that takes the base URL for testing purposes,
	// or we export the Client/BaseURL.
	
	// Intentionally referencing a struct that doesn't exist yet to cause compilation failure (Red)
	embedder := embedding.NewVertexEmbedder("my-project", "us-central1", &MockTokenSource{})
	embedder.BaseURL = server.URL // Seam for testing

	texts := []string{"Hello world", "Graph database"}
	embeddings, err := embedder.EmbedBatch(texts)

	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(embeddings) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(embeddings))
	}

	if len(embeddings[0]) != 3 || embeddings[0][0] != 0.1 {
		t.Errorf("Unexpected values in first embedding: %v", embeddings[0])
	}
}
