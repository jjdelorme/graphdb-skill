package leiden

import (
	"fmt"
	"testing"
)

func BenchmarkLeidenEngine500Nodes(b *testing.B) {
	nodes := make([]string, 500)
	for i := 0; i < 500; i++ {
		nodes[i] = fmt.Sprintf("n%d", i)
	}

	edges := make([]RawEdge, 0, 5000)
	for i := 0; i < 500; i++ {
		for j := i + 1; j < i+10 && j < 500; j++ {
			edges = append(edges, RawEdge{
				SourceID: nodes[i],
				TargetID: nodes[j],
				Type:     "CALLS",
			})
		}
	}

	engine := NewDefaultEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Partition(nodes, edges)
	}
}

func BenchmarkLeidenEngine2000Nodes(b *testing.B) {
	nodes := make([]string, 2000)
	for i := 0; i < 2000; i++ {
		nodes[i] = fmt.Sprintf("n%d", i)
	}

	edges := make([]RawEdge, 0, 20000)
	for i := 0; i < 2000; i++ {
		for j := i + 1; j < i+10 && j < 2000; j++ {
			edges = append(edges, RawEdge{
				SourceID: nodes[i],
				TargetID: nodes[j],
				Type:     "CALLS",
			})
		}
	}

	engine := NewDefaultEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Partition(nodes, edges)
	}
}
