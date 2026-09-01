package graph

type Node struct {
	ID         string                 `json:"id"`
	Label      string                 `json:"label"`
	Properties map[string]interface{} `json:"properties"`
}

type Edge struct {
	SourceID   string                 `json:"sourceId"`
	TargetID   string                 `json:"targetId"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type Path struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

// Node Labels
const (
	LabelStructuralCommunity = "StructuralCommunity"
	LabelSharedBoundary      = "SharedBoundary"
	LabelCrossCuttingHub     = "CrossCuttingHub"
	LabelCodeElement         = "CodeElement"
	LabelFunction            = "Function"
	LabelClass               = "Class"
	LabelFile                = "File"
	LabelDomain              = "Domain"
	LabelFeature             = "Feature"
)

// Relationship Types
const (
	RelInCommunity      = "IN_COMMUNITY"
	RelBridges          = "BRIDGES"
	RelInfrastructureOf = "INFRASTRUCTURE_OF"
	RelCalls            = "CALLS"
	RelInherits         = "INHERITS"
	RelUsesGlobal       = "USES_GLOBAL"
	RelCoChanged        = "CO_CHANGED"
	RelImplements       = "IMPLEMENTS"
	RelParentOf         = "PARENT_OF"
	RelDefinedIn        = "DEFINED_IN"
)
