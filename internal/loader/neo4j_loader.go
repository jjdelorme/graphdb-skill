package loader

import (
	"context"
	"fmt"
	"graphdb/internal/graph"
	"graphdb/internal/logger"
	"graphdb/internal/progress"
	"log"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jLoader handles batch loading of graph data into Neo4j.
type Neo4jLoader struct {
	Driver              neo4j.DriverWithContext
	DBName              string
	EmbeddingDimensions int
}

// NewNeo4jLoader creates a new loader instance.
func NewNeo4jLoader(driver neo4j.DriverWithContext, dbName string, embeddingDimensions int) *Neo4jLoader {
	return &Neo4jLoader{
		Driver:              driver,
		DBName:              dbName,
		EmbeddingDimensions: embeddingDimensions,
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
			logger.Query.Printf("Neo4j Loader Query: %s", query)
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
			logger.Query.Printf("Neo4j Loader Query: %s", query)
			return tx.Run(ctx, query, map[string]any{"batch": batch})
		})
		if err != nil {
			return fmt.Errorf("failed to load edges for type %s: %w", relType, err)
		}
	}

	return nil
}

// ApplyConstraints creates uniqueness constraints and indexes.
func (l *Neo4jLoader) ApplyConstraints(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)
	
	var errs []string

	for _, query := range l.getConstraints() {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			logger.Query.Printf("Neo4j Loader Query: %s", query)
			return tx.Run(ctx, query, nil)
		})
		if err != nil {
			// Log the error but continue to the next constraint
			msg := fmt.Sprintf("failed to apply constraint: %s. Error: %v", query, err)
			if !progress.IsAnyActive() {
				log.Printf("WARN: %s", msg)
			}
			errs = append(errs, msg)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors applying constraints:\n%s", len(errs), strings.Join(errs, "\n"))
	}

	return nil
}

// UpdateGraphState updates the commit hash.
func (l *Neo4jLoader) UpdateGraphState(ctx context.Context, commit string, dir string) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	query := buildGraphStateQuery(dir != "")
	params := map[string]any{
		"commit": commit,
	}
	if dir != "" {
		params["dir"] = dir
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		logger.Query.Printf("Neo4j Loader Query: %s", query)
		return tx.Run(ctx, query, params)
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
			MERGE (n:CodeElement {id: row.id})
			SET n += row
			SET n:%s
		`, SanitizeLabel(label))
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
			MATCH (source:CodeElement {id: row.sourceId})
			MATCH (target:CodeElement) WHERE target.id = row.targetId OR target.fqn = row.targetId
			MERGE (source)-[r:%s]->(target)
		`, SanitizeLabel(relType))
}

func buildGraphStateQuery(isDirSpecific bool) string {
	if isDirSpecific {
		return `
			MERGE (s:Metadata {dir: $dir})
			SET s.commit = $commit, s.updatedAt = datetime()
		`
	}
	return `
		MERGE (s:GraphState)
		SET s.commit = $commit, s.updatedAt = datetime()
	`
}

func (l *Neo4jLoader) getConstraints() []string {
	dims := l.EmbeddingDimensions
	if dims <= 0 {
		dims = 768 // Fallback default
	}

	return []string{
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:File) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Function) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Class) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:CodeElement) REQUIRE n.id IS UNIQUE",
		"CREATE INDEX IF NOT EXISTS FOR (n:CodeElement) ON (n.fqn)",
		"CREATE INDEX IF NOT EXISTS FOR (n:Function) ON (n.name)",
		"CREATE INDEX IF NOT EXISTS FOR (n:Function) ON (n.is_test)",
		"CREATE INDEX IF NOT EXISTS FOR (n:File) ON (n.file)",
		"CREATE INDEX IF NOT EXISTS FOR (n:File) ON (n.is_test)",
		// Vector Indexes (restored from Node.js implementation)
		fmt.Sprintf(`CREATE VECTOR INDEX feature_embeddings IF NOT EXISTS
		FOR (n:Feature) ON (n.embedding)
		OPTIONS {indexConfig: {
			`+"`vector.dimensions`"+`: %d,
			`+"`vector.similarity_function`"+`: 'cosine'
		}}`, dims),
		fmt.Sprintf(`CREATE VECTOR INDEX function_embeddings IF NOT EXISTS
		FOR (n:Function) ON (n.embedding)
		OPTIONS {indexConfig: {
			`+"`vector.dimensions`"+`: %d,
			`+"`vector.similarity_function`"+`: 'cosine'
		}}`, dims),
	}
}

func SanitizeLabel(label string) string {
	var sb strings.Builder
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// WipeDatabase clears all data from the database using a batched transaction delete
// and falling back to a non-transactional delete if the batch query fails.
func (l *Neo4jLoader) WipeDatabase(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: l.DBName})
	defer session.Close(ctx)

	// 1. Try transactional batch delete first (requires auto-commit transaction session.Run)
	queryBatch := `CALL { MATCH (n) DETACH DELETE n } IN TRANSACTIONS OF 10000 ROWS`
	result, err := session.Run(ctx, queryBatch, nil)
	if err == nil {
		if _, err = result.Consume(ctx); err == nil {
			return nil
		}
	}

	// 2. Fallback to basic DETACH DELETE if transactional delete fails/unsupported
	logger.Query.Printf("Warning: loader batch transactional wipe failed: %v. Falling back to non-transactional wipe.", err)
	resultFallback, err := session.Run(ctx, `MATCH (n) DETACH DELETE n`, nil)
	if err != nil {
		return fmt.Errorf("failed to wipe database: %w", err)
	}
	_, err = resultFallback.Consume(ctx)
	if err != nil {
		return fmt.Errorf("failed to consume fallback wipe query: %w", err)
	}
	return nil
}
