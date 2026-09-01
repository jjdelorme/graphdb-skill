package leiden

// CalculateQuality calculates the exact CPM quality value for a given partition on graph g:
// H_CPM = sum_{c in P} [ e(c,c) - gamma * binom(w(c), 2) ]
func CalculateQuality(g *Graph, partition []int, gamma float64) float64 {
	n := len(g.NodeIDs)
	if n == 0 {
		return 0.0
	}

	// Calculate community node weights and internal edge weights
	commWeights := make(map[int]float64)
	commInternalEdges := make(map[int]float64)

	for i := 0; i < n; i++ {
		c := partition[i]
		commWeights[c] += g.NodeWeights[i]
		commInternalEdges[c] += g.SelfLoops[i]

		for _, nb := range g.Neighbors[i] {
			if i < nb.Target && partition[nb.Target] == c {
				commInternalEdges[c] += nb.Weight
			}
		}
	}

	var quality float64
	for c, w := range commWeights {
		e := commInternalEdges[c]
		binom := (w * (w - 1.0)) / 2.0
		quality += e - (gamma * binom)
	}

	return quality
}

// DeltaH computes the net quality gain of moving node v from srcComm to dstComm:
// DeltaH(v: C_src -> C_dst) = E(v, C_dst) - E(v, C_src) - gamma * w(v) * [ w(C_dst) - w(C_src) + w(v) ]
func DeltaH(
	v int,
	srcComm int,
	dstComm int,
	eSrc float64,
	eDst float64,
	wV float64,
	wSrc float64,
	wDst float64,
	gamma float64,
) float64 {
	if srcComm == dstComm {
		return 0.0
	}

	// If moving to a new empty singleton community (dstComm == -1 or wDst == 0)
	if dstComm == -1 || wDst == 0 {
		return -eSrc + gamma*wV*(wSrc-wV)
	}

	return eDst - eSrc - gamma*wV*(wDst-wSrc+wV)
}
