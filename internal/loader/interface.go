package loader

import (
	"context"
	"graphdb/internal/graph"
)

// Loader defines the interface for batch loading graph data.
type Loader interface {
	BatchLoadNodes(ctx context.Context, nodes []graph.Node) error
	BatchLoadEdges(ctx context.Context, edges []graph.Edge) error
	ApplyConstraints(ctx context.Context) error
	UpdateGraphState(ctx context.Context, commit string, dir string) error
	WipeDatabase(ctx context.Context) error
}
