package leiden

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestLeidenDisjointCliques(t *testing.T) {
	// 3 completely disjoint cliques of 10 nodes each
	nodes := []string{}
	edges := []RawEdge{}

	for c := 0; c < 3; c++ {
		for i := 0; i < 10; i++ {
			id := fmt.Sprintf("c%d_n%d", c, i)
			nodes = append(nodes, id)
		}
		for i := 0; i < 10; i++ {
			for j := i + 1; j < 10; j++ {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("c%d_n%d", c, i),
					TargetID: fmt.Sprintf("c%d_n%d", c, j),
					Type:     "CALLS",
					Weight:   1.0,
				})
			}
		}
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)
	rng := rand.New(rand.NewSource(42))
	partition := LeidenClustering(g, 0.1, 50, rng)

	// Verify all nodes in each clique have the exact same community ID, and different across cliques
	c0Comm := partition[g.IDToIndex["c0_n0"]]
	c1Comm := partition[g.IDToIndex["c1_n0"]]
	c2Comm := partition[g.IDToIndex["c2_n0"]]

	if c0Comm == c1Comm || c1Comm == c2Comm || c0Comm == c2Comm {
		t.Errorf("Expected 3 distinct community IDs, got c0=%d, c1=%d, c2=%d", c0Comm, c1Comm, c2Comm)
	}

	for c := 0; c < 3; c++ {
		expectedComm := partition[g.IDToIndex[fmt.Sprintf("c%d_n0", c)]]
		for i := 0; i < 10; i++ {
			actualComm := partition[g.IDToIndex[fmt.Sprintf("c%d_n%d", c, i)]]
			if actualComm != expectedComm {
				t.Errorf("Node c%d_n%d has comm %d, expected %d", c, i, actualComm, expectedComm)
			}
		}
	}
}

func TestLeidenBarbellGraph(t *testing.T) {
	// Two 15-node cliques connected by a single weak bridge edge
	nodes := []string{}
	edges := []RawEdge{}

	for i := 0; i < 15; i++ {
		nodes = append(nodes, fmt.Sprintf("left_%d", i))
		nodes = append(nodes, fmt.Sprintf("right_%d", i))
	}

	for i := 0; i < 15; i++ {
		for j := i + 1; j < 15; j++ {
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("left_%d", i), TargetID: fmt.Sprintf("left_%d", j), Type: "CALLS"})
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("right_%d", i), TargetID: fmt.Sprintf("right_%d", j), Type: "CALLS"})
		}
	}

	// Single bridge edge
	edges = append(edges, RawEdge{SourceID: "left_0", TargetID: "right_0", Type: "REFERENCES", Weight: 0.5})

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)
	rng := rand.New(rand.NewSource(42))
	partition := LeidenClustering(g, 0.2, 50, rng)

	leftComm := partition[g.IDToIndex["left_1"]]
	rightComm := partition[g.IDToIndex["right_1"]]

	if leftComm == rightComm {
		t.Fatalf("Expected left and right cliques to be separated into distinct communities")
	}

	for i := 0; i < 15; i++ {
		if partition[g.IDToIndex[fmt.Sprintf("left_%d", i)]] != leftComm {
			t.Errorf("Left node %d placed in wrong community", i)
		}
		if partition[g.IDToIndex[fmt.Sprintf("right_%d", i)]] != rightComm {
			t.Errorf("Right node %d placed in wrong community", i)
		}
	}
}

func TestLeidenResolutionLimitImmunity(t *testing.T) {
	// Ring of 10 cliques of 15 nodes each, connected by single inter-clique edges.
	// Standard Modularity merges adjacent small cliques in large graphs (Resolution Limit).
	// CPM Leiden preserves each distinct module cleanly.
	numCliques := 10
	cliqueSize := 15
	nodes := []string{}
	edges := []RawEdge{}

	for c := 0; c < numCliques; c++ {
		for i := 0; i < cliqueSize; i++ {
			nodes = append(nodes, fmt.Sprintf("clique_%d_n_%d", c, i))
		}
		// Internal clique edges (weight 1.0)
		for i := 0; i < cliqueSize; i++ {
			for j := i + 1; j < cliqueSize; j++ {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("clique_%d_n_%d", c, i),
					TargetID: fmt.Sprintf("clique_%d_n_%d", c, j),
					Type:     "CALLS",
					Weight:   1.0,
				})
			}
		}
		// Inter-clique ring bridge (weight 0.2)
		nextC := (c + 1) % numCliques
		edges = append(edges, RawEdge{
			SourceID: fmt.Sprintf("clique_%d_n_0", c),
			TargetID: fmt.Sprintf("clique_%d_n_0", nextC),
			Type:     "REFERENCES",
			Weight:   0.2,
		})
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)
	rng := rand.New(rand.NewSource(42))
	partition := LeidenClustering(g, 0.3, 50, rng)

	uniqueComm := make(map[int]struct{})
	for _, comm := range partition {
		uniqueComm[comm] = struct{}{}
	}

	if len(uniqueComm) != numCliques {
		t.Fatalf("CPM Resolution Limit Immunity Failed: Expected %d communities, got %d", numCliques, len(uniqueComm))
	}
}

func TestLeidenDeterminism10Runs(t *testing.T) {
	// 50-node random-like multi-cluster graph
	nodes := []string{}
	for i := 0; i < 60; i++ {
		nodes = append(nodes, fmt.Sprintf("node_%d", i))
	}

	edges := []RawEdge{}
	// 3 loose clusters with cross links
	for c := 0; c < 3; c++ {
		start := c * 20
		end := start + 20
		for i := start; i < end; i++ {
			for j := i + 1; j < end; j++ {
				if (i+j)%3 == 0 {
					edges = append(edges, RawEdge{SourceID: fmt.Sprintf("node_%d", i), TargetID: fmt.Sprintf("node_%d", j), Type: "CALLS"})
				}
			}
		}
	}
	// Cross edges
	edges = append(edges, RawEdge{SourceID: "node_5", TargetID: "node_25", Type: "REFERENCES"})
	edges = append(edges, RawEdge{SourceID: "node_25", TargetID: "node_45", Type: "REFERENCES"})

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)

	var baseline []int
	for run := 0; run < 10; run++ {
		rng := rand.New(rand.NewSource(42)) // Fixed seed
		part := LeidenClustering(g, 0.1, 50, rng)
		if run == 0 {
			baseline = part
		} else {
			if !reflect.DeepEqual(baseline, part) {
				t.Fatalf("Run %d yielded different partition than baseline! Determinism violation.", run)
			}
		}
	}
}

func TestLeidenMultiLevelCoarsening(t *testing.T) {
	// 4 connected cliques of 5 nodes each with bridge edges
	// Tests that multi-level meta-graph aggregation coarsens across multiple levels
	numCliques := 4
	cliqueSize := 5
	nodes := []string{}
	edges := []RawEdge{}

	for c := 0; c < numCliques; c++ {
		for i := 0; i < cliqueSize; i++ {
			nodes = append(nodes, fmt.Sprintf("c%d_n%d", c, i))
		}
		for i := 0; i < cliqueSize; i++ {
			for j := i + 1; j < cliqueSize; j++ {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("c%d_n%d", c, i),
					TargetID: fmt.Sprintf("c%d_n%d", c, j),
					Type:     "CALLS",
					Weight:   1.0,
				})
			}
		}
		if c > 0 {
			edges = append(edges, RawEdge{
				SourceID: fmt.Sprintf("c%d_n%d", c-1, cliqueSize-1),
				TargetID: fmt.Sprintf("c%d_n0", c),
				Type:     "REFERENCES",
				Weight:   0.5,
			})
		}
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)

	// Low gamma -> should coarsen through meta-nodes into 1 community
	rngLow := rand.New(rand.NewSource(42))
	partLow := LeidenClustering(g, 1e-6, 50, rngLow)
	uniqueLow := make(map[int]struct{})
	for _, c := range partLow {
		uniqueLow[c] = struct{}{}
	}
	if len(uniqueLow) != 1 {
		t.Errorf("Expected 1 merged community for gamma=1e-6 on connected cliques, got %d", len(uniqueLow))
	}
	if len(partLow) != len(nodes) {
		t.Fatalf("Expected partition length %d, got %d", len(nodes), len(partLow))
	}

	// High gamma -> should detect 4 separate communities
	rngHigh := rand.New(rand.NewSource(42))
	partHigh := LeidenClustering(g, 0.5, 50, rngHigh)
	uniqueHigh := make(map[int]struct{})
	for _, c := range partHigh {
		uniqueHigh[c] = struct{}{}
	}
	if len(uniqueHigh) != 4 {
		t.Errorf("Expected 4 distinct communities for gamma=0.5 on connected cliques, got %d", len(uniqueHigh))
	}
}
