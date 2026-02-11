package query

import (
	"context"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/graph"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jProvider implements GraphProvider using the official Neo4j Go driver.
type Neo4jProvider struct {
	driver neo4j.DriverWithContext
	ctx    context.Context
}

// NewNeo4jProvider creates a new connection to Neo4j.
func NewNeo4jProvider(cfg config.Config) (*Neo4jProvider, error) {
	auth := neo4j.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, "")

	driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	ctx := context.Background()
	// Verify connectivity
	if err := driver.VerifyConnectivity(ctx); err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("failed to verify connectivity to neo4j: %w", err)
	}

	return &Neo4jProvider{
		driver: driver,
		ctx:    ctx,
	}, nil
}

// Close closes the Neo4j driver connection.
func (p *Neo4jProvider) Close() error {
	return p.driver.Close(p.ctx)
}

// FindNode finds a node by label and property.
func (p *Neo4jProvider) FindNode(label string, property string, value string) (*graph.Node, error) {
	// TODO: Implement in Phase 2.2+
	return nil, nil
}

// Traverse traverses the graph from a start node.
func (p *Neo4jProvider) Traverse(startNodeID string, relationship string, direction Direction, depth int) ([]*graph.Path, error) {
	// TODO: Implement in Phase 2.2+
	return nil, nil
}

// SearchFeatures searches for nodes using vector embeddings and structure.
func (p *Neo4jProvider) SearchFeatures(embedding []float32, limit int) ([]*FeatureResult, error) {
	query := `
		CALL db.index.vector.queryNodes('function_embeddings', $limit, $embedding)
		YIELD node, score
		RETURN node.label as label, score, properties(node) as props
	`

	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"limit":     limit,
		"embedding": embedding,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search: %w", err)
	}

	features := make([]*FeatureResult, 0, len(result.Records))
	for _, record := range result.Records {
		label, _, err := neo4j.GetRecordValue[string](record, "label")
		if err != nil {
			continue // Should not happen if query is correct
		}
		score, _, _ := neo4j.GetRecordValue[float64](record, "score")
		
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")

		// Reconstruct node (simplified)
		node := &graph.Node{
			Label: label,
			Properties: make(map[string]any),
		}
		for k, v := range props {
			node.Properties[k] = v
		}

		features = append(features, &FeatureResult{
			Node:  node,
			Score: float32(score),
		})
	}

	return features, nil
}

// GetNeighbors retrieves the dependencies (functions, globals) of a node.
func (p *Neo4jProvider) GetNeighbors(nodeID string, depth int) (*NeighborResult, error) {
	query := fmt.Sprintf(`
		MATCH (f:Function {label: $func})
		
		// 1. Direct & Transitive Globals
		OPTIONAL MATCH path = (f)-[:CALLS*0..%d]->(callee)-[:USES_GLOBAL]->(g:Global)
		WITH f, collect(DISTINCT {
			dependency: g.label, 
			type: 'Global', 
			via: [n in nodes(path) WHERE n.label <> $func | n.label]
		}) as globals

		// 2. Direct Function Calls
		MATCH (f)-[:CALLS]->(d:Function)
		WITH globals, collect(DISTINCT {dependency: d.label, type: 'Function', labels: labels(d)}) as funcs
		
		RETURN globals + funcs as dependencies
	`, depth)

	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"func": nodeID,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetNeighbors query: %w", err)
	}

	if len(result.Records) == 0 {
		// Node not found or no dependencies?
		// Check if node exists first?
		// For now, return empty result if no records, but the query returns aggregated list so it should return 1 record if f exists, or 0 if f doesn't exist?
		// "MATCH (f:Function {label: $func})" acts as a filter. If f doesn't exist, it returns 0 records.
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	dependenciesRaw, _, err := neo4j.GetRecordValue[[]any](result.Records[0], "dependencies")
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies from record: %w", err)
	}

	deps := make([]Dependency, 0, len(dependenciesRaw))
	for _, raw := range dependenciesRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		
		dep := Dependency{
			Name: item["dependency"].(string),
			Type: item["type"].(string),
		}

		if viaRaw, ok := item["via"]; ok && viaRaw != nil {
			if viaList, ok := viaRaw.([]any); ok {
				via := make([]string, len(viaList))
				for i, v := range viaList {
					via[i] = v.(string)
				}
				// Clean up: if via is empty, it might be direct.
				// The JS code did: if (item.type === 'Global' && item.via.length === 0) delete item.via;
				// In Go, empty slice is fine.
				dep.Via = via
			}
		}
		deps = append(deps, dep)
	}

	return &NeighborResult{
		// Node: ... we didn't fetch the node properties, just dependencies. 
		// The interface says Node *graph.Node. We might want to fetch it or leave it nil.
		// For now, let's leave it nil or populate with minimal info.
		Node:         &graph.Node{Label: nodeID}, 
		Dependencies: deps,
	}, nil
}

// GetCallers retrieves the callers of a node.
func (p *Neo4jProvider) GetCallers(nodeID string) ([]string, error) {
	query := `
		MATCH (caller:Function)-[:CALLS]->(f:Function {label: $func})
		RETURN collect(DISTINCT caller.label) as callers
	`

	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"func": nodeID,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetCallers query: %w", err)
	}

	if len(result.Records) == 0 {
		return []string{}, nil
	}

	callersRaw, _, err := neo4j.GetRecordValue[[]any](result.Records[0], "callers")
	if err != nil {
		return nil, fmt.Errorf("failed to get callers from record: %w", err)
	}

	callers := make([]string, len(callersRaw))
	for i, raw := range callersRaw {
		callers[i] = raw.(string)
	}

	return callers, nil
}

// GetImpact analyzes the impact of changing a node (reverse dependencies).
func (p *Neo4jProvider) GetImpact(nodeID string, depth int) (*ImpactResult, error) {
	// Construct dynamic query for variable path length
	query := fmt.Sprintf(`
		MATCH (caller:Function)-[:CALLS*1..%d]->(f:Function {label: $nodeID}) 
		RETURN DISTINCT caller.label as caller, caller.ui_contaminated as contaminated
	`, depth)

	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"nodeID": nodeID,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetImpact query: %w", err)
	}

	callers := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		label, _, err := neo4j.GetRecordValue[string](record, "caller")
		if err != nil {
			continue
		}
		contaminated, _, _ := neo4j.GetRecordValue[bool](record, "contaminated")

		node := &graph.Node{
			Label: label,
			Properties: map[string]any{
				"ui_contaminated": contaminated,
			},
		}
		callers = append(callers, node)
	}

	return &ImpactResult{
		Target:  &graph.Node{Label: nodeID},
		Callers: callers,
		// Paths: nil, // Not implementing paths yet as per requirement, just callers
	}, nil
}

// GetGlobals identifies global variable usage.
func (p *Neo4jProvider) GetGlobals(nodeID string) (*GlobalUsageResult, error) {
	query := `
		MATCH (f:Function {label: $nodeID})-[:USES_GLOBAL]->(g:Global) 
		RETURN g.label as name, g.file as defined_in
	`

	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"nodeID": nodeID,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetGlobals query: %w", err)
	}

	globals := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		name, _, err := neo4j.GetRecordValue[string](record, "name")
		if err != nil {
			continue
		}
		file, _, _ := neo4j.GetRecordValue[string](record, "defined_in")

		node := &graph.Node{
			Label: name,
			Properties: map[string]any{
				"file": file,
			},
		}
		globals = append(globals, node)
	}

	return &GlobalUsageResult{
		Target:  &graph.Node{Label: nodeID},
		Globals: globals,
	}, nil
}

// SuggestSeams suggests architectural seams (clusters).
func (p *Neo4jProvider) SuggestSeams() (*SeamResult, error) {
	// TODO: Implement in Phase 2.4+
	return nil, nil
}
