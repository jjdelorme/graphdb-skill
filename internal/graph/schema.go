package graph

type Node struct {
	ID         string                 `json:"id"`
	Label      string                 `json:"label"`
	Properties map[string]interface{} `json:"properties"`
}

type Edge struct {
	SourceID string `json:"sourceId"`
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
}

type Path struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}
