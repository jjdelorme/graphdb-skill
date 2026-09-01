package leiden

import (
	"fmt"
	"testing"
)

func TestEvaluatePenaltyFunction(t *testing.T) {
	// Partition with 100 nodes:
	// 50 nodes in comm 0 (size 50, ideal)
	// 40 nodes in comm 1 (size 40, ideal)
	// 10 nodes in comm 2 (size 10 < 30, small)
	partition := make([]int, 100)
	for i := 0; i < 50; i++ {
		partition[i] = 0
	}
	for i := 50; i < 90; i++ {
		partition[i] = 1
	}
	for i := 90; i < 100; i++ {
		partition[i] = 2
	}

	penalty := EvaluatePenalty(partition, 30, 250)
	// FracSmall = 10 / 100 = 0.10
	// FracLarge = 0 / 100 = 0.0
	// MaxRatio = 50 / 100 = 0.50
	// Score = 0.35 * 0.10 + 0.45 * 0.0 + 0.20 * 0.50 = 0.035 + 0.10 = 0.135
	expected := 0.135
	if fmt.Sprintf("%.4f", penalty) != fmt.Sprintf("%.4f", expected) {
		t.Errorf("Expected penalty %f, got %f", expected, penalty)
	}
}

func TestSearchOptimalGamma(t *testing.T) {
	// Multi-module synthetic graph: 4 modules of 40 nodes each
	nodes := []string{}
	edges := []RawEdge{}

	for m := 0; m < 4; m++ {
		for i := 0; i < 40; i++ {
			nodes = append(nodes, fmt.Sprintf("m%d_n%d", m, i))
		}
		// Dense internal module edges (ring + internal chords)
		for i := 0; i < 40; i++ {
			edges = append(edges, RawEdge{
				SourceID: fmt.Sprintf("m%d_n%d", m, i),
				TargetID: fmt.Sprintf("m%d_n%d", m, (i+1)%40),
				Type:     "CALLS",
			})
			for j := i + 2; j < 40; j += 3 {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("m%d_n%d", m, i),
					TargetID: fmt.Sprintf("m%d_n%d", m, j),
					Type:     "CALLS",
				})
			}
		}
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)
	cfg := DefaultConfig()
	cfg.MinCommunitySize = 30
	cfg.MaxCommunitySize = 250
	cfg.ResolutionSteps = 6

	optGamma, partition := SearchOptimalGamma(g, cfg, 42)
	if optGamma <= 0 {
		t.Errorf("Expected positive gamma, got %f", optGamma)
	}

	uniqueComm := make(map[int]int)
	for _, c := range partition {
		uniqueComm[c]++
	}

	if len(uniqueComm) < 4 {
		t.Errorf("Expected at least 4 communities, got %d", len(uniqueComm))
	}

	for commID, count := range uniqueComm {
		if count < 25 || count > 60 {
			t.Errorf("Community %d size %d outside expected target range [25, 60]", commID, count)
		}
	}
}

func TestRecursiveSubClusteringOversized(t *testing.T) {
	// Build a 300-node graph that contains two 150-node internal sub-components
	// connected by a few edges.
	nodes := []string{}
	edges := []RawEdge{}

	for i := 0; i < 300; i++ {
		nodes = append(nodes, fmt.Sprintf("node_%d", i))
	}

	// Sub-cluster 1: nodes 0..149
	for i := 0; i < 150; i++ {
		for j := i + 1; j < 150; j++ {
			if (i*7+j)%5 == 0 {
				edges = append(edges, RawEdge{SourceID: fmt.Sprintf("node_%d", i), TargetID: fmt.Sprintf("node_%d", j), Type: "CALLS"})
			}
		}
	}

	// Sub-cluster 2: nodes 150..299
	for i := 150; i < 300; i++ {
		for j := i + 1; j < 300; j++ {
			if (i*7+j)%5 == 0 {
				edges = append(edges, RawEdge{SourceID: fmt.Sprintf("node_%d", i), TargetID: fmt.Sprintf("node_%d", j), Type: "CALLS"})
			}
		}
	}

	// A few weak bridge edges between sub-clusters
	edges = append(edges, RawEdge{SourceID: "node_10", TargetID: "node_160", Type: "REFERENCES"})
	edges = append(edges, RawEdge{SourceID: "node_50", TargetID: "node_200", Type: "REFERENCES"})

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)

	// Force initial partition to treat all 300 nodes as 1 single oversized community
	initialPart := make([]int, 300) // all in comm 0

	cfg := DefaultConfig()
	cfg.MaxCommunitySize = 200 // Threshold < 300 so it triggers sub-clustering
	cfg.MaxHierDepth = 3

	subPartition := SubClusterOversized(g, initialPart, 0.05, cfg, 42, 0)

	uniqueComm := make(map[int]int)
	for _, c := range subPartition {
		uniqueComm[c]++
	}

	if len(uniqueComm) < 2 {
		t.Fatalf("Expected sub-clustering to split 300-node community into >= 2 sub-communities, got %d", len(uniqueComm))
	}

	for commID, count := range uniqueComm {
		if count > cfg.MaxCommunitySize {
			t.Errorf("Sub-community %d size %d still exceeds MaxCommunitySize %d", commID, count, cfg.MaxCommunitySize)
		}
	}
}
