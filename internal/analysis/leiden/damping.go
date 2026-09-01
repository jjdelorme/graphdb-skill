package leiden

import (
	"math"
	"sort"
	"strings"
)

// ResolveBaseWeight maps edge types and raw weights to base coupling strengths.
func (m EdgeWeightMatrix) ResolveBaseWeight(edgeType string, rawWeight float64) float64 {
	cleanType := strings.ToUpper(strings.TrimSpace(edgeType))
	switch cleanType {
	case "CALLS":
		factor := m.CallsWeight
		if factor <= 0 {
			factor = 1.0
		}
		if rawWeight > 0 {
			return rawWeight * factor
		}
		return factor
	case "CONTAINS":
		factor := m.ContainsWeight
		if factor <= 0 {
			factor = 0.8
		}
		if rawWeight > 0 {
			return rawWeight * factor
		}
		return factor
	case "INHERITS", "IMPLEMENTS":
		factor := m.InheritsWeight
		if factor <= 0 {
			factor = 0.9
		}
		if rawWeight > 0 {
			return rawWeight * factor
		}
		return factor
	case "USES_GLOBAL", "WRITES_GLOBAL", "READS_GLOBAL":
		factor := m.UsesGlobalWeight
		if factor <= 0 {
			factor = 0.7
		}
		if rawWeight > 0 {
			return rawWeight * factor
		}
		return factor
	case "CO_CHANGED":
		if rawWeight > 0 {
			w := 0.2 * rawWeight
			if w > 2.0 {
				return 2.0
			}
			return w
		}
		if m.CoChangedWeight > 0 {
			return m.CoChangedWeight
		}
		return 0.6
	case "REFERENCES", "TYPE_USAGE", "INSTANTIATES":
		factor := m.ReferencesWeight
		if factor <= 0 {
			factor = 0.5
		}
		if rawWeight > 0 {
			return rawWeight * factor
		}
		return factor
	case "IMPLICIT_SEMANTIC":
		if rawWeight >= 0.85 {
			return 0.5 * rawWeight
		}
		if rawWeight > 0 && rawWeight < 0.85 {
			return 0.0 // Below semantic affinity threshold
		}
		if m.ImplicitSemanticWeight > 0 {
			return m.ImplicitSemanticWeight
		}
		return 0.4
	case "TESTS":
		return 0.0 // Suppressed during structural clustering
	default:
		if rawWeight > 0 {
			return rawWeight
		}
		if m.CallsWeight > 0 {
			return m.CallsWeight
		}
		return 1.0
	}
}

// pairKey returns a canonical undirected key for node index pair (u, v).
func pairKey(u, v int) uint64 {
	if u < v {
		return (uint64(u) << 32) | uint64(v)
	}
	return (uint64(v) << 32) | uint64(u)
}

// BuildGraph constructs an in-memory Graph, consolidating multi-edges and optionally applying inverse-degree damping.
func BuildGraph(nodes []string, edges []RawEdge, matrix EdgeWeightMatrix, suppressHubs bool) *Graph {
	idToIndex := make(map[string]int, len(nodes))
	nodeIDs := make([]string, 0, len(nodes))

	for _, id := range nodes {
		if _, exists := idToIndex[id]; !exists {
			idx := len(nodeIDs)
			idToIndex[id] = idx
			nodeIDs = append(nodeIDs, id)
		}
	}

	// Also ensure any node referenced in edges is indexed
	for _, e := range edges {
		if _, exists := idToIndex[e.SourceID]; !exists {
			idx := len(nodeIDs)
			idToIndex[e.SourceID] = idx
			nodeIDs = append(nodeIDs, e.SourceID)
		}
		if _, exists := idToIndex[e.TargetID]; !exists {
			idx := len(nodeIDs)
			idToIndex[e.TargetID] = idx
			nodeIDs = append(nodeIDs, e.TargetID)
		}
	}

	n := len(nodeIDs)
	nodeWeights := make([]float64, n)
	selfLoops := make([]float64, n)
	for i := 0; i < n; i++ {
		nodeWeights[i] = 1.0
	}

	// Consolidate base multi-edges
	edgeBaseMap := make(map[uint64]float64)
	distinctNeighbors := make([]map[int]struct{}, n)
	for i := 0; i < n; i++ {
		distinctNeighbors[i] = make(map[int]struct{})
	}

	for _, e := range edges {
		srcIdx, srcOk := idToIndex[e.SourceID]
		dstIdx, dstOk := idToIndex[e.TargetID]
		if !srcOk || !dstOk {
			continue
		}

		baseWt := matrix.ResolveBaseWeight(e.Type, e.Weight)
		if baseWt <= 0 {
			continue
		}

		if srcIdx == dstIdx {
			selfLoops[srcIdx] += baseWt
		} else {
			key := pairKey(srcIdx, dstIdx)
			edgeBaseMap[key] += baseWt
			distinctNeighbors[srcIdx][dstIdx] = struct{}{}
			distinctNeighbors[dstIdx][srcIdx] = struct{}{}
		}
	}

	// Calculate unweighted degrees
	degrees := make([]int, n)
	for i := 0; i < n; i++ {
		degrees[i] = len(distinctNeighbors[i])
	}

	// Build adjacency list with effective weights in deterministic key order
	keys := make([]uint64, 0, len(edgeBaseMap))
	for k := range edgeBaseMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	neighbors := make([][]WeightedNeighbor, n)
	var totalWeight float64

	for _, key := range keys {
		baseWt := edgeBaseMap[key]
		u := int(key >> 32)
		v := int(key & 0xFFFFFFFF)

		effWt := baseWt
		if suppressHubs {
			degU := float64(degrees[u])
			degV := float64(degrees[v])
			if degU < 1.0 {
				degU = 1.0
			}
			if degV < 1.0 {
				degV = 1.0
			}
			dampingFactor := 1.0 / (math.Log(1.0+degU) * math.Log(1.0+degV))
			effWt = baseWt * dampingFactor
		}

		neighbors[u] = append(neighbors[u], WeightedNeighbor{Target: v, Weight: effWt})
		neighbors[v] = append(neighbors[v], WeightedNeighbor{Target: u, Weight: effWt})
		totalWeight += effWt
	}

	for i := 0; i < n; i++ {
		totalWeight += selfLoops[i]
		sort.Slice(neighbors[i], func(a, b int) bool {
			return neighbors[i][a].Target < neighbors[i][b].Target
		})
	}

	return &Graph{
		NodeIDs:     nodeIDs,
		IDToIndex:   idToIndex,
		NodeWeights: nodeWeights,
		SelfLoops:   selfLoops,
		Neighbors:   neighbors,
		Degrees:     degrees,
		TotalWeight: totalWeight,
	}
}
