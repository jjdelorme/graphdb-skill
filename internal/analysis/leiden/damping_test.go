package leiden

import (
	"fmt"
	"math"
	"testing"
)

func TestEdgeWeightMatrixResolution(t *testing.T) {
	matrix := DefaultEdgeWeightMatrix()

	tests := []struct {
		edgeType  string
		rawWeight float64
		expected  float64
	}{
		{"CALLS", 0.0, 1.0},
		{"CONTAINS", 0.0, 0.8},
		{"INHERITS", 0.0, 0.9},
		{"IMPLEMENTS", 0.0, 0.9},
		{"USES_GLOBAL", 0.0, 0.7},
		{"WRITES_GLOBAL", 0.0, 0.7},
		{"READS_GLOBAL", 0.0, 0.7},
		{"REFERENCES", 0.0, 0.5},
		{"TYPE_USAGE", 0.0, 0.5},
		{"INSTANTIATES", 0.0, 0.5},
		{"CO_CHANGED", 0.0, 0.6},
		{"CO_CHANGED", 5.0, 1.0},  // 0.2 * 5.0 = 1.0
		{"CO_CHANGED", 20.0, 2.0}, // capped at 2.0
		{"IMPLICIT_SEMANTIC", 0.0, 0.4},
		{"IMPLICIT_SEMANTIC", 0.9, 0.45}, // 0.5 * 0.9 = 0.45
		{"IMPLICIT_SEMANTIC", 0.8, 0.0},  // below 0.85 threshold -> 0.0
		{"TESTS", 1.0, 0.0},              // suppressed
		{"CUSTOM_TYPE", 2.5, 2.5},
	}

	for _, tc := range tests {
		actual := matrix.ResolveBaseWeight(tc.edgeType, tc.rawWeight)
		if math.Abs(actual-tc.expected) > 1e-9 {
			t.Errorf("For %s (raw %f), expected %f, got %f", tc.edgeType, tc.rawWeight, tc.expected, actual)
		}
	}
}

func TestInverseDegreeDamping(t *testing.T) {
	// Construct a graph with a hub connected to 10 nodes (deg = 10)
	// and 2 leaf nodes connected only to each other (deg = 1)
	nodes := []string{"hub"}
	for i := 1; i <= 10; i++ {
		nodes = append(nodes, fmt.Sprintf("leaf%d", i))
	}
	nodes = append(nodes, "lowA", "lowB")

	edges := []RawEdge{}
	for i := 1; i <= 10; i++ {
		edges = append(edges, RawEdge{SourceID: "hub", TargetID: fmt.Sprintf("leaf%d", i), Type: "CALLS"})
	}
	// Add edge between lowA and lowB
	edges = append(edges, RawEdge{SourceID: "lowA", TargetID: "lowB", Type: "CALLS"})

	// Graph with damping
	gDamped := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), true)

	// Hub has deg 10, leaf1 has deg 1
	// LowA has deg 1, LowB has deg 1
	hubIdx := gDamped.IDToIndex["hub"]
	leaf1Idx := gDamped.IDToIndex["leaf1"]
	lowAIdx := gDamped.IDToIndex["lowA"]
	lowBIdx := gDamped.IDToIndex["lowB"]

	var hubLeafWt float64
	for _, nb := range gDamped.Neighbors[hubIdx] {
		if nb.Target == leaf1Idx {
			hubLeafWt = nb.Weight
			break
		}
	}

	var lowABWt float64
	for _, nb := range gDamped.Neighbors[lowAIdx] {
		if nb.Target == lowBIdx {
			lowABWt = nb.Weight
			break
		}
	}

	// Theoretical calculation:
	// deg(hub) = 10 -> ln(1+10) = ln(11) = 2.397895
	// deg(leaf1) = 1 -> ln(1+1) = ln(2) = 0.693147
	// Damping factor = 1 / (2.397895 * 0.693147) = 1 / 1.662094 = 0.60165
	// deg(lowA) = 1, deg(lowB) = 1 -> ln(2)*ln(2) = 0.480453 -> Damping factor = 1 / 0.480453 = 2.08137
	if hubLeafWt >= lowABWt {
		t.Errorf("Expected hub edge to be significantly more damped than low-degree edge! hubLeafWt=%f, lowABWt=%f", hubLeafWt, lowABWt)
	}

	expectedHubLeaf := 1.0 / (math.Log(11.0) * math.Log(2.0))
	expectedLowAB := 1.0 / (math.Log(2.0) * math.Log(2.0))

	if math.Abs(hubLeafWt-expectedHubLeaf) > 1e-6 {
		t.Errorf("Hub edge weight calculation mismatch! expected=%f, got=%f", expectedHubLeaf, hubLeafWt)
	}
	if math.Abs(lowABWt-expectedLowAB) > 1e-6 {
		t.Errorf("Low-degree edge weight calculation mismatch! expected=%f, got=%f", expectedLowAB, lowABWt)
	}
}

func TestMultiEdgeConsolidation(t *testing.T) {
	nodes := []string{"A", "B"}
	edges := []RawEdge{
		{SourceID: "A", TargetID: "B", Type: "CALLS"},      // 1.0
		{SourceID: "B", TargetID: "A", Type: "REFERENCES"}, // 0.5
		{SourceID: "A", TargetID: "B", Type: "CONTAINS"},   // 0.8
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)
	aIdx := g.IDToIndex["A"]

	if len(g.Neighbors[aIdx]) != 1 {
		t.Fatalf("Expected 1 consolidated neighbor for A, got %d", len(g.Neighbors[aIdx]))
	}

	wt := g.Neighbors[aIdx][0].Weight
	expected := 1.0 + 0.5 + 0.8 // 2.3
	if math.Abs(wt-expected) > 1e-9 {
		t.Errorf("Expected consolidated weight %f, got %f", expected, wt)
	}
}
