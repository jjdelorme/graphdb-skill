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
