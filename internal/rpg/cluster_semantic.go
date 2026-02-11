package rpg

import (
	"fmt"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"math"
	"math/rand"
	"strings"
)

// EmbeddingClusterer groups functions by semantic similarity of their
// atomic_features using embedding vectors and K-Means clustering.
type EmbeddingClusterer struct {
	Embedder      embedding.Embedder
	MaxIterations int // K-Means iterations; 0 defaults to 50
}

func (c *EmbeddingClusterer) Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	// If too few nodes, put them all in one cluster
	if len(nodes) <= 3 {
		return map[string][]graph.Node{domain + "-core": nodes}, nil
	}

	// 1. Build embedding for each function from its atomic_features
	texts := make([]string, len(nodes))
	for i, n := range nodes {
		if af, ok := n.Properties["atomic_features"].([]string); ok && len(af) > 0 {
			texts[i] = strings.Join(af, ", ")
		} else if afAny, ok := n.Properties["atomic_features"].([]interface{}); ok && len(afAny) > 0 {
			parts := make([]string, len(afAny))
			for j, v := range afAny {
				parts[j] = fmt.Sprintf("%v", v)
			}
			texts[i] = strings.Join(parts, ", ")
		} else {
			// Fallback: use function name
			name, _ := n.Properties["name"].(string)
			texts[i] = name
		}
		if texts[i] == "" {
			texts[i] = n.ID
		}
	}

	embeddings, err := c.Embedder.EmbedBatch(texts)
	if err != nil {
		return nil, fmt.Errorf("embedding for clustering failed: %w", err)
	}

	// 2. Determine K: target 3-8 functions per cluster
	k := len(nodes) / 5
	if k < 2 {
		k = 2
	}
	if k > len(nodes)/2 {
		k = len(nodes) / 2
	}

	// 3. Run K-Means
	maxIter := c.MaxIterations
	if maxIter <= 0 {
		maxIter = 50
	}
	assignments := kmeans(embeddings, k, maxIter)

	// 4. Group nodes by cluster assignment
	clusters := make(map[string][]graph.Node)
	for i, clusterIdx := range assignments {
		key := fmt.Sprintf("%s-cluster-%d", domain, clusterIdx)
		clusters[key] = append(clusters[key], nodes[i])
	}

	return clusters, nil
}

// kmeans runs K-Means clustering on a set of vectors.
// Returns a slice of cluster assignments (one per input vector).
func kmeans(vectors [][]float32, k int, maxIterations int) []int {
	n := len(vectors)
	if n == 0 || k <= 0 {
		return nil
	}
	dim := len(vectors[0])

	// Initialize centroids using K-Means++ initialization
	centroids := kmeansppInit(vectors, k)

	assignments := make([]int, n)

	for iter := 0; iter < maxIterations; iter++ {
		changed := false

		// Assign each vector to nearest centroid
		for i, v := range vectors {
			nearest := 0
			nearestDist := cosineDistance(v, centroids[0])
			for j := 1; j < k; j++ {
				d := cosineDistance(v, centroids[j])
				if d < nearestDist {
					nearestDist = d
					nearest = j
				}
			}
			if assignments[i] != nearest {
				assignments[i] = nearest
				changed = true
			}
		}

		if !changed {
			break
		}

		// Recompute centroids
		newCentroids := make([][]float32, k)
		counts := make([]int, k)
		for j := 0; j < k; j++ {
			newCentroids[j] = make([]float32, dim)
		}
		for i, v := range vectors {
			c := assignments[i]
			counts[c]++
			for d := 0; d < dim; d++ {
				newCentroids[c][d] += v[d]
			}
		}
		for j := 0; j < k; j++ {
			if counts[j] > 0 {
				for d := 0; d < dim; d++ {
					newCentroids[j][d] /= float32(counts[j])
				}
				centroids[j] = newCentroids[j]
			}
		}
	}

	return assignments
}

// kmeansppInit selects initial centroids using K-Means++ algorithm.
func kmeansppInit(vectors [][]float32, k int) [][]float32 {
	n := len(vectors)
	centroids := make([][]float32, 0, k)

	// Pick first centroid randomly
	first := rand.Intn(n)
	centroids = append(centroids, vectors[first])

	for len(centroids) < k {
		// Compute distances to nearest centroid
		dists := make([]float64, n)
		total := 0.0
		for i, v := range vectors {
			minDist := math.MaxFloat64
			for _, c := range centroids {
				d := float64(cosineDistance(v, c))
				if d < minDist {
					minDist = d
				}
			}
			dists[i] = minDist * minDist
			total += dists[i]
		}

		// Weighted random selection
		r := rand.Float64() * total
		cumulative := 0.0
		chosen := 0
		for i, d := range dists {
			cumulative += d
			if cumulative >= r {
				chosen = i
				break
			}
		}
		centroids = append(centroids, vectors[chosen])
	}

	return centroids
}

// cosineDistance returns 1 - cosine_similarity between two vectors.
func cosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	sim := dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	return 1.0 - sim
}
