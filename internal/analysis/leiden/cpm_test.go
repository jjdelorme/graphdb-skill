package leiden

import (
	"math"
	"testing"
)

func TestCPMQualityExactCalculation(t *testing.T) {
	// Build a simple 4-node graph: 2 disjoint pairs (0-1) and (2-3) with edge weights 1.0
	nodes := []string{"n0", "n1", "n2", "n3"}
	edges := []RawEdge{
		{SourceID: "n0", TargetID: "n1", Type: "CALLS", Weight: 1.0},
		{SourceID: "n2", TargetID: "n3", Type: "CALLS", Weight: 1.0},
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)

	gamma := 0.1
	// Partition P1: {0,1} and {2,3} -> 2 communities of size 2, each with 1 internal edge
	// H_CPM = (1.0 - 0.1 * binom(2, 2)) + (1.0 - 0.1 * binom(2, 2))
	//       = (1.0 - 0.1 * 1) + (1.0 - 0.1 * 1) = 0.9 + 0.9 = 1.8
	p1 := []int{0, 0, 1, 1}
	q1 := CalculateQuality(g, p1, gamma)
	expected1 := 1.8
	if math.Abs(q1-expected1) > 1e-9 {
		t.Errorf("Expected quality %f, got %f", expected1, q1)
	}

	// Partition P2: All in 1 community of size 4, with 2 internal edges
	// H_CPM = 2.0 - 0.1 * binom(4, 2) = 2.0 - 0.1 * 6 = 2.0 - 0.6 = 1.4
	p2 := []int{0, 0, 0, 0}
	q2 := CalculateQuality(g, p2, gamma)
	expected2 := 1.4
	if math.Abs(q2-expected2) > 1e-9 {
		t.Errorf("Expected quality %f, got %f", expected2, q2)
	}

	// Partition P3: All singletons (size 1, 0 internal edges)
	// H_CPM = 4 * (0 - 0.1 * binom(1, 2)) = 0
	p3 := []int{0, 1, 2, 3}
	q3 := CalculateQuality(g, p3, gamma)
	expected3 := 0.0
	if math.Abs(q3-expected3) > 1e-9 {
		t.Errorf("Expected quality %f, got %f", expected3, q3)
	}
}

func TestCPMDeltaHConsistency(t *testing.T) {
	// Verify that DeltaH(v: C_src -> C_dst) == Quality(P_after) - Quality(P_before)
	nodes := []string{"n0", "n1", "n2", "n3", "n4"}
	edges := []RawEdge{
		{SourceID: "n0", TargetID: "n1", Type: "CALLS", Weight: 1.0},
		{SourceID: "n1", TargetID: "n2", Type: "CALLS", Weight: 1.0},
		{SourceID: "n0", TargetID: "n2", Type: "CALLS", Weight: 1.0},
		{SourceID: "n2", TargetID: "n3", Type: "CALLS", Weight: 0.5},
		{SourceID: "n3", TargetID: "n4", Type: "CALLS", Weight: 1.0},
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)
	gamma := 0.15

	// Initial partition: n0,n1,n2 in Comm 0; n3,n4 in Comm 1
	pBefore := []int{0, 0, 0, 1, 1}
	qBefore := CalculateQuality(g, pBefore, gamma)

	// Move n2 (idx 2) from Comm 0 to Comm 1
	pAfter := []int{0, 0, 1, 1, 1}
	qAfter := CalculateQuality(g, pAfter, gamma)
	actualDelta := qAfter - qBefore

	// Calculate DeltaH using formula
	v := 2
	srcComm := 0
	dstComm := 1
	wV := g.NodeWeights[v]
	wSrc := 3.0 // n0, n1, n2
	wDst := 2.0 // n3, n4

	// Edge weights from v (n2) to Comm 0: n0 (1.0) + n1 (1.0) = 2.0
	eSrc := 2.0
	// Edge weights from v (n2) to Comm 1: n3 (0.5) = 0.5
	eDst := 0.5

	formulaDelta := DeltaH(v, srcComm, dstComm, eSrc, eDst, wV, wSrc, wDst, gamma)

	if math.Abs(actualDelta-formulaDelta) > 1e-9 {
		t.Errorf("DeltaH formula mismatch! actual=%f, formula=%f", actualDelta, formulaDelta)
	}

	// Test move to singleton empty community
	pSingleton := []int{0, 0, 2, 1, 1}
	qSingleton := CalculateQuality(g, pSingleton, gamma)
	actualSingletonDelta := qSingleton - qBefore

	formulaSingletonDelta := DeltaH(v, srcComm, -1, eSrc, 0.0, wV, wSrc, 0.0, gamma)
	if math.Abs(actualSingletonDelta-formulaSingletonDelta) > 1e-9 {
		t.Errorf("DeltaH empty singleton mismatch! actual=%f, formula=%f", actualSingletonDelta, formulaSingletonDelta)
	}
}
