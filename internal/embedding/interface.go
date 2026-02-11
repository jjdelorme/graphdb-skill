package embedding

// Embedder defines the interface for generating vector embeddings from text.
type Embedder interface {
	// EmbedBatch generates embeddings for a batch of texts.
	// Returns a slice of float32 slices, where each inner slice is the embedding for the corresponding text.
	EmbedBatch(texts []string) ([][]float32, error)
}
