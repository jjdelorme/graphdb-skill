package query

import (
	"context"
	"fmt"
	"graphdb/internal/graph"
	"graphdb/internal/loader"
	"graphdb/internal/logger"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const structuralTopologyBatchSize = 500

// ClearStructuralTopology removes all :StructuralCommunity nodes and clears
// :SharedBoundary and :CrossCuttingHub labels and properties from CodeElement nodes.
func (p *Neo4jProvider) ClearStructuralTopology() error {
	queries := []string{
		"// Clear Structural Communities\nMATCH (c:StructuralCommunity) DETACH DELETE c",
		"// Remove SharedBoundary labels and properties\nMATCH (n:SharedBoundary) REMOVE n:SharedBoundary, n.is_shared_boundary, n.bpr_max, n.boundary_community_count",
		"// Remove CrossCuttingHub labels and properties\nMATCH (n:CrossCuttingHub) REMOVE n:CrossCuttingHub, n.is_cross_cutting_hub, n.degree, n.hub_score",
	}

	for _, q := range queries {
		if _, err := p.executeQuery(q, nil); err != nil {
			return fmt.Errorf("failed to clear structural topology with query %q: %w", q, err)
		}
	}
	return nil
}

// UpdateStructuralTopology batch writes structural communities, memberships,
// shared boundaries, and infrastructure hubs using chunked UNWIND transactions.
func (p *Neo4jProvider) UpdateStructuralTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	if len(nodes) == 0 && len(edges) == 0 {
		return nil
	}

	ctx := p.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Group nodes by category
	var communityNodes []*graph.Node
	var sharedBoundaryNodes []*graph.Node
	var hubNodes []*graph.Node
	var otherNodes []*graph.Node

	for _, n := range nodes {
		switch n.Label {
		case "StructuralCommunity":
			communityNodes = append(communityNodes, n)
		case "SharedBoundary":
			sharedBoundaryNodes = append(sharedBoundaryNodes, n)
		case "CrossCuttingHub":
			hubNodes = append(hubNodes, n)
		default:
			otherNodes = append(otherNodes, n)
		}
	}

	// 2. Persist StructuralCommunity nodes
	if err := p.batchWriteCommunityNodes(ctx, communityNodes, structuralTopologyBatchSize); err != nil {
		return fmt.Errorf("failed to write structural community nodes: %w", err)
	}

	// 3. Persist SharedBoundary nodes
	if err := p.batchWriteSharedBoundaryNodes(ctx, sharedBoundaryNodes, structuralTopologyBatchSize); err != nil {
		return fmt.Errorf("failed to write shared boundary nodes: %w", err)
	}

	// 4. Persist CrossCuttingHub nodes
	if err := p.batchWriteHubNodes(ctx, hubNodes, structuralTopologyBatchSize); err != nil {
		return fmt.Errorf("failed to write cross cutting hub nodes: %w", err)
	}

	// 5. Persist any other nodes if provided
	if len(otherNodes) > 0 {
		if err := p.batchWriteNodes(ctx, otherNodes, structuralTopologyBatchSize); err != nil {
			return fmt.Errorf("failed to write generic nodes: %w", err)
		}
	}

	// 6. Group edges by type
	edgeGroups := make(map[string][]*graph.Edge)
	for _, e := range edges {
		edgeGroups[e.Type] = append(edgeGroups[e.Type], e)
	}

	for relType, groupEdges := range edgeGroups {
		switch relType {
		case "IN_COMMUNITY":
			if err := p.batchWriteInCommunityEdges(ctx, groupEdges, structuralTopologyBatchSize); err != nil {
				return fmt.Errorf("failed to write IN_COMMUNITY edges: %w", err)
			}
		case "BRIDGES":
			if err := p.batchWriteBridgesEdges(ctx, groupEdges, structuralTopologyBatchSize); err != nil {
				return fmt.Errorf("failed to write BRIDGES edges: %w", err)
			}
		case "INFRASTRUCTURE_OF":
			if err := p.batchWriteInfrastructureEdges(ctx, groupEdges, structuralTopologyBatchSize); err != nil {
				return fmt.Errorf("failed to write INFRASTRUCTURE_OF edges: %w", err)
			}
		default:
			if err := p.batchWriteGenericEdges(ctx, relType, groupEdges, structuralTopologyBatchSize); err != nil {
				return fmt.Errorf("failed to write %s edges: %w", relType, err)
			}
		}
	}

	return nil
}

func (p *Neo4jProvider) batchWriteCommunityNodes(ctx context.Context, nodes []*graph.Node, batchSize int) error {
	if len(nodes) == 0 {
		return nil
	}

	totalBatches := (len(nodes) + batchSize - 1) / batchSize

	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		chunk := nodes[i:end]
		batchNum := (i / batchSize) + 1

		batch := make([]map[string]any, 0, len(chunk))
		for _, n := range chunk {
			row := map[string]any{
				"id":                  n.ID,
				"name":                n.ID,
				"gamma":               0.0,
				"size":                int64(0),
				"density":             0.0,
				"internal_edge_count": int64(0),
				"bpr_avg":             0.0,
			}
			if n.Properties != nil {
				for k, v := range n.Properties {
					row[k] = v
				}
			}
			batch = append(batch, row)
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				UNWIND $batch AS row
				MERGE (c:StructuralCommunity {id: row.id})
				SET c.name = row.name,
				    c.gamma = row.gamma,
				    c.size = row.size,
				    c.density = row.density,
				    c.internal_edge_count = row.internal_edge_count,
				    c.bpr_avg = row.bpr_avg,
				    c.created_at = datetime()
			`
			logger.Query.Printf("Query: Batch Write Structural Communities (%d nodes)", len(batch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write community batch %d/%d: %w", batchNum, totalBatches, err)
		}
		logger.Query.Printf("Writing structural topology: communities batch %d/%d (%d/%d)", batchNum, totalBatches, end, len(nodes))
	}

	return nil
}

func (p *Neo4jProvider) batchWriteSharedBoundaryNodes(ctx context.Context, nodes []*graph.Node, batchSize int) error {
	if len(nodes) == 0 {
		return nil
	}

	totalBatches := (len(nodes) + batchSize - 1) / batchSize

	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		chunk := nodes[i:end]
		batchNum := (i / batchSize) + 1

		batch := make([]map[string]any, 0, len(chunk))
		for _, n := range chunk {
			row := map[string]any{
				"id":                       n.ID,
				"bpr_max":                  0.0,
				"boundary_community_count": 0,
			}
			if n.Properties != nil {
				for k, v := range n.Properties {
					row[k] = v
				}
			}
			batch = append(batch, row)
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				UNWIND $batch AS row
				MATCH (n:CodeElement {id: row.id})
				SET n:SharedBoundary,
				    n.is_shared_boundary = true,
				    n.bpr_max = row.bpr_max,
				    n.boundary_community_count = row.boundary_community_count
			`
			logger.Query.Printf("Query: Batch Write Shared Boundaries (%d nodes)", len(batch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write shared boundary batch %d/%d: %w", batchNum, totalBatches, err)
		}
		logger.Query.Printf("Writing structural topology: shared boundaries batch %d/%d (%d/%d)", batchNum, totalBatches, end, len(nodes))
	}

	return nil
}

func (p *Neo4jProvider) batchWriteHubNodes(ctx context.Context, nodes []*graph.Node, batchSize int) error {
	if len(nodes) == 0 {
		return nil
	}

	totalBatches := (len(nodes) + batchSize - 1) / batchSize

	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		chunk := nodes[i:end]
		batchNum := (i / batchSize) + 1

		batch := make([]map[string]any, 0, len(chunk))
		for _, n := range chunk {
			row := map[string]any{
				"id":        n.ID,
				"degree":    0,
				"hub_score": 0.0,
			}
			if n.Properties != nil {
				for k, v := range n.Properties {
					row[k] = v
				}
			}
			batch = append(batch, row)
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				UNWIND $batch AS row
				MATCH (n:CodeElement {id: row.id})
				SET n:CrossCuttingHub,
				    n.is_cross_cutting_hub = true,
				    n.degree = row.degree,
				    n.hub_score = row.hub_score
			`
			logger.Query.Printf("Query: Batch Write Cross Cutting Hubs (%d nodes)", len(batch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write cross cutting hub batch %d/%d: %w", batchNum, totalBatches, err)
		}
		logger.Query.Printf("Writing structural topology: hubs batch %d/%d (%d/%d)", batchNum, totalBatches, end, len(nodes))
	}

	return nil
}

func (p *Neo4jProvider) batchWriteInCommunityEdges(ctx context.Context, edges []*graph.Edge, batchSize int) error {
	if len(edges) == 0 {
		return nil
	}

	totalBatches := (len(edges) + batchSize - 1) / batchSize

	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		chunk := edges[i:end]
		batchNum := (i / batchSize) + 1

		batch := make([]map[string]any, 0, len(chunk))
		for _, e := range chunk {
			batch = append(batch, map[string]any{
				"sourceId": e.SourceID,
				"targetId": e.TargetID,
			})
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				UNWIND $batch AS row
				MATCH (n:CodeElement {id: row.sourceId})
				MATCH (c:StructuralCommunity {id: row.targetId})
				MERGE (n)-[r:IN_COMMUNITY]->(c)
			`
			logger.Query.Printf("Query: Batch Write IN_COMMUNITY Edges (%d edges)", len(batch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write IN_COMMUNITY edge batch %d/%d: %w", batchNum, totalBatches, err)
		}
		logger.Query.Printf("Writing structural topology: IN_COMMUNITY batch %d/%d (%d/%d)", batchNum, totalBatches, end, len(edges))
	}

	return nil
}

func (p *Neo4jProvider) batchWriteBridgesEdges(ctx context.Context, edges []*graph.Edge, batchSize int) error {
	if len(edges) == 0 {
		return nil
	}

	totalBatches := (len(edges) + batchSize - 1) / batchSize

	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		chunk := edges[i:end]
		batchNum := (i / batchSize) + 1

		batch := make([]map[string]any, 0, len(chunk))
		for _, e := range chunk {
			ratio := 0.0
			if e.Properties != nil {
				if r, ok := e.Properties["ratio"].(float64); ok {
					ratio = r
				}
			}
			batch = append(batch, map[string]any{
				"sourceId": e.SourceID,
				"targetId": e.TargetID,
				"ratio":    ratio,
			})
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				UNWIND $batch AS row
				MATCH (n:CodeElement {id: row.sourceId})
				MATCH (c:StructuralCommunity {id: row.targetId})
				MERGE (n)-[r:BRIDGES]->(c)
				SET r.ratio = row.ratio
			`
			logger.Query.Printf("Query: Batch Write BRIDGES Edges (%d edges)", len(batch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write BRIDGES edge batch %d/%d: %w", batchNum, totalBatches, err)
		}
		logger.Query.Printf("Writing structural topology: BRIDGES batch %d/%d (%d/%d)", batchNum, totalBatches, end, len(edges))
	}

	return nil
}

func (p *Neo4jProvider) batchWriteInfrastructureEdges(ctx context.Context, edges []*graph.Edge, batchSize int) error {
	if len(edges) == 0 {
		return nil
	}

	totalBatches := (len(edges) + batchSize - 1) / batchSize

	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		chunk := edges[i:end]
		batchNum := (i / batchSize) + 1

		batch := make([]map[string]any, 0, len(chunk))
		for _, e := range chunk {
			affinity := 0.0
			callCount := 0
			if e.Properties != nil {
				if a, ok := e.Properties["affinity"].(float64); ok {
					affinity = a
				}
				if cc, ok := e.Properties["call_count"].(int); ok {
					callCount = cc
				}
			}
			batch = append(batch, map[string]any{
				"sourceId":  e.SourceID,
				"targetId":  e.TargetID,
				"affinity":   affinity,
				"call_count": callCount,
			})
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				UNWIND $batch AS row
				MATCH (n:CodeElement {id: row.sourceId})
				MATCH (c:StructuralCommunity {id: row.targetId})
				MERGE (n)-[r:INFRASTRUCTURE_OF]->(c)
				SET r.affinity = row.affinity,
				    r.call_count = row.call_count
			`
			logger.Query.Printf("Query: Batch Write INFRASTRUCTURE_OF Edges (%d edges)", len(batch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write INFRASTRUCTURE_OF edge batch %d/%d: %w", batchNum, totalBatches, err)
		}
		logger.Query.Printf("Writing structural topology: INFRASTRUCTURE_OF batch %d/%d (%d/%d)", batchNum, totalBatches, end, len(edges))
	}

	return nil
}

func (p *Neo4jProvider) batchWriteGenericEdges(ctx context.Context, relType string, edges []*graph.Edge, batchSize int) error {
	if len(edges) == 0 {
		return nil
	}

	sanitizedRel := loader.SanitizeLabel(relType)
	totalBatches := (len(edges) + batchSize - 1) / batchSize

	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		chunk := edges[i:end]
		batchNum := (i / batchSize) + 1

		batch := make([]map[string]any, 0, len(chunk))
		for _, e := range chunk {
			batch = append(batch, map[string]any{
				"sourceId": e.SourceID,
				"targetId": e.TargetID,
			})
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := fmt.Sprintf(`
				UNWIND $batch AS row
				MATCH (source:CodeElement {id: row.sourceId})
				MATCH (target:CodeElement {id: row.targetId})
				MERGE (source)-[r:%s]->(target)
			`, sanitizedRel)
			logger.Query.Printf("Query: Batch Write Generic [%s] Edges (%d edges)", relType, len(batch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write %s edge batch %d/%d: %w", relType, batchNum, totalBatches, err)
		}
		logger.Query.Printf("Writing structural topology: %s batch %d/%d (%d/%d)", relType, batchNum, totalBatches, end, len(edges))
	}

	return nil
}
