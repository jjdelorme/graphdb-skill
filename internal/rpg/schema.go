package rpg

import "graphdb/internal/graph"

type Feature struct {
	ID          string
	Name        string
	Description string
	Embedding   []float32
	ScopePath   string
	Children    []*Feature
}

func (f *Feature) ToNode() graph.Node {
	return graph.Node{
		ID:    f.ID,
		Label: "Feature",
		Properties: map[string]interface{}{
			"name":        f.Name,
			"description": f.Description,
			"embedding":   f.Embedding,
			"scope_path":  f.ScopePath,
		},
	}
}

func Flatten(features []Feature, edges []graph.Edge) ([]graph.Node, []graph.Edge) {
	var nodes []graph.Node
	var allEdges []graph.Edge
	allEdges = append(allEdges, edges...)

	var visit func(f *Feature)
	visit = func(f *Feature) {
		nodes = append(nodes, f.ToNode())
		for _, child := range f.Children {
			visit(child)
		}
	}

	for i := range features {
		visit(&features[i])
	}

	return nodes, allEdges
}
