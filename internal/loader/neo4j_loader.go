package loader

import (
	"context"
	"fmt"
	"graphdb/internal/graph"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jLoader handles batch loading of graph data into Neo4j.
type Neo4jLoader struct {
	Driver neo4j.DriverWithContext
	DBName string
}

// NewNeo4jLoader creates a new loader instance.
func NewNeo4jLoader(driver neo4j.DriverWithContext, dbName string) *Neo4jLoader {
	return &Neo4jLoader{
		Driver: driver,
		DBName: dbName,
	}
}

// BatchLoadNodes loads a batch of nodes using UNWIND.
func (l *Neo4jLoader) BatchLoadNodes(ctx context.Context, nodes []graph.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	batches := groupNodesByLabel(nodes)

	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	for label, batch := range batches {
		query := buildNodeQuery(label)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, query, map[string]any{"batch": batch})
		})
		if err != nil {
			return fmt.Errorf("failed to load nodes for label %s: %w", label, err)
		}
	}

	return nil
}

// BatchLoadEdges loads a batch of edges using UNWIND.
func (l *Neo4jLoader) BatchLoadEdges(ctx context.Context, edges []graph.Edge) error {
	if len(edges) == 0 {
		return nil
	}

	batches := groupEdgesByType(edges)

	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	for relType, batch := range batches {
		query := buildEdgeQuery(relType)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, query, map[string]any{"batch": batch})
		})
		if err != nil {
			return fmt.Errorf("failed to load edges for type %s: %w", relType, err)
		}
	}

	return nil
}

// Wipe deletes all data from the database.
func (l *Neo4jLoader) Wipe(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	query := buildWipeQuery()
	// Using APOC if available is better: "CALL apoc.periodic.iterate('MATCH (n) RETURN n', 'DETACH DELETE n', {batchSize:1000})".
	// But without APOC, we just run a simple delete. For large DBs this might timeout, but it matches the JS impl.
	
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, nil)
	})
	return err
}

// ApplyConstraints creates uniqueness constraints and indexes.
func (l *Neo4jLoader) ApplyConstraints(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)
	
	constraints := []string{
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:File) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Function) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Class) REQUIRE n.id IS UNIQUE",
		"CREATE INDEX IF NOT EXISTS FOR (n:Function) ON (n.name)",
		"CREATE INDEX IF NOT EXISTS FOR (n:File) ON (n.file)",
	}

	for _, query := range constraints {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, query, nil)
		})
		if err != nil {
			return fmt.Errorf("failed to apply constraint '%s': %w", query, err)
		}
	}
	return nil
}

// UpdateGraphState updates the commit hash.
func (l *Neo4jLoader) UpdateGraphState(ctx context.Context, commit string) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	query := buildGraphStateQuery()
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, map[string]any{"commit": commit})
	})
	return err
}

// Helpers extracted for testing
func groupNodesByLabel(nodes []graph.Node) map[string][]map[string]any {
	batches := make(map[string][]map[string]any)
	for _, n := range nodes {
		label := n.Label
		if label == "" {
			label = "Generic"
		}
		
		props := make(map[string]any)
		for k, v := range n.Properties {
			props[k] = v
		}
		props["id"] = n.ID
		
		batches[label] = append(batches[label], props)
	}
	return batches
}

func buildNodeQuery(label string) string {
	return fmt.Sprintf(`
			UNWIND $batch AS row
			MERGE (n:%s {id: row.id})
			SET n += row
		`, sanitizeLabel(label))
}

func groupEdgesByType(edges []graph.Edge) map[string][]map[string]any {
	batches := make(map[string][]map[string]any)
	for _, e := range edges {
		relType := e.Type
		if relType == "" {
			relType = "RELATED_TO"
		}
		
		row := map[string]any{
			"sourceId": e.SourceID,
			"targetId": e.TargetID,
		}
		batches[relType] = append(batches[relType], row)
	}
	return batches
}

func buildEdgeQuery(relType string) string {
	return fmt.Sprintf(`
			UNWIND $batch AS row
			MATCH (source {id: row.sourceId})
			MATCH (target {id: row.targetId})
			MERGE (source)-[r:%s]->(target)
		`, sanitizeLabel(relType))
}

func buildWipeQuery() string {
	return "MATCH (n) DETACH DELETE n"
}

func buildGraphStateQuery() string {
	return `
		MERGE (s:GraphState)
		SET s.commit = $commit, s.updatedAt = datetime()
	`
}

func sanitizeLabel(label string) string {
	return strings.ReplaceAll(label, "`", "")
}
