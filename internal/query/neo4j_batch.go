package query

import (
	"context"
	"fmt"
	"graphdb/internal/graph"
	"graphdb/internal/loader"
	"graphdb/internal/logger"
	"log"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

const featureTopologyBatchSize = 500

// GetUnextractedFunctions returns functions that haven't had atomic features extracted yet.
func (p *Neo4jProvider) GetUnextractedFunctions(limit int) ([]*graph.Node, error) {
	query := `
		// Get Unextracted Functions
		MATCH (n:Function)
		WHERE n.atomic_features IS NULL AND n.file IS NOT NULL AND n.start_line IS NOT NULL
		RETURN n.id as id, n.name as name, n.file as file, n.start_line as start, n.end_line as end
		LIMIT $limit
	`
	result, err := p.executeQuery(query, map[string]any{
		"limit": limit,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get unextracted functions: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		name, _, _ := neo4j.GetRecordValue[string](record, "name")
		file, _, _ := neo4j.GetRecordValue[string](record, "file")
		start, _, _ := neo4j.GetRecordValue[int64](record, "start")
		end, _, _ := neo4j.GetRecordValue[int64](record, "end")

		nodes = append(nodes, &graph.Node{
			ID:    id,
			Label: "Function",
			Properties: map[string]any{
				"name":       name,
				"file":       file,
				"start_line": start,
				"end_line":   end,
			},
		})
	}
	return nodes, nil
}

// CountUnextractedFunctions returns the total number of functions without atomic features.
func (p *Neo4jProvider) CountUnextractedFunctions() (int64, error) {
	query := `
		// Count Unextracted Functions
		MATCH (n:Function)
		WHERE n.atomic_features IS NULL AND n.file IS NOT NULL AND n.start_line IS NOT NULL
		RETURN count(n) as total
	`
	result, err := p.executeQuery(query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count unextracted functions: %w", err)
	}
	if len(result.Records) == 0 {
		return 0, nil
	}
	total, _, _ := neo4j.GetRecordValue[int64](result.Records[0], "total")
	return total, nil
}

// UpdateAtomicFeatures saves the extracted atomic features for a node.
func (p *Neo4jProvider) UpdateAtomicFeatures(id string, features []string, isVolatile bool) error {
	query := `
		// Update Atomic Features
		MATCH (n:Function {id: $id})
		SET n.atomic_features = $features, n.is_volatile = $isVolatile
	`
	_, err := p.executeQuery(query, map[string]any{
		"id":         id,
		"features":   features,
		"isVolatile": isVolatile,
	})

	if err != nil {
		return fmt.Errorf("failed to update atomic features for %s: %w", id, err)
	}
	return nil
}

// GetUnembeddedNodes returns functions and features that lack an embedding.
func (p *Neo4jProvider) GetUnembeddedNodes(limit int) ([]*graph.Node, error) {
	query := `
		// Get Unembedded Nodes
		MATCH (n)
		WHERE (n:Function OR n:Feature) AND n.embedding IS NULL
		RETURN n.id as id, labels(n)[0] as label, properties(n) as props
		LIMIT $limit
	`
	result, err := p.executeQuery(query, map[string]any{
		"limit": limit,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get unembedded nodes: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		label, _, _ := neo4j.GetRecordValue[string](record, "label")
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")

		nodes = append(nodes, &graph.Node{
			ID:         id,
			Label:      label,
			Properties: sanitizeProperties(props),
		})
	}
	return nodes, nil
}

// CountUnembeddedNodes returns the total number of functions and features that lack an embedding.
func (p *Neo4jProvider) CountUnembeddedNodes() (int64, error) {
	query := `
		// Count Unembedded Nodes
		MATCH (n)
		WHERE (n:Function OR n:Feature) AND n.embedding IS NULL
		RETURN count(n) as total
	`
	result, err := p.executeQuery(query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count unembedded nodes: %w", err)
	}
	if len(result.Records) == 0 {
		return 0, nil
	}
	total, _, _ := neo4j.GetRecordValue[int64](result.Records[0], "total")
	return total, nil
}

// UpdateEmbeddings updates the embedding vector for a node.
func (p *Neo4jProvider) UpdateEmbeddings(id string, embedding []float32) error {
	query := `
		// Update Embeddings
		MATCH (n {id: $id})
		SET n.embedding = $embedding
	`
	_, err := p.executeQuery(query, map[string]any{
		"id":        id,
		"embedding": embedding,
	})

	if err != nil {
		return fmt.Errorf("failed to update embedding for %s: %w", id, err)
	}
	return nil
}

// GetEmbeddingsOnly returns all IDs and their embeddings from the graph.
func (p *Neo4jProvider) GetEmbeddingsOnly() (map[string][]float32, error) {
	query := `
		// Get Embeddings Only
		MATCH (n)
		WHERE (n:Feature OR (n:Function AND coalesce(n.is_test, false) = false AND NOT (size(n.atomic_features) = 1 AND n.atomic_features[0] = 'unknown')))
		  AND n.embedding IS NOT NULL
		RETURN n.id as id, n.embedding as embedding
	`
	result, err := p.executeQuery(query, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings: %w", err)
	}

	embeddings := make(map[string][]float32, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")

		// Neo4j returns float arrays as []any containing float64
		embeddingRaw, _, _ := neo4j.GetRecordValue[[]any](record, "embedding")

		emb := make([]float32, len(embeddingRaw))
		for i, v := range embeddingRaw {
			if f64, ok := v.(float64); ok {
				emb[i] = float32(f64)
			}
		}

		embeddings[id] = emb
	}
	return embeddings, nil
}

// GetFunctionMetadata returns all functions with minimal properties (id, file, start_line, end_line, atomic_features) for clustering.
func (p *Neo4jProvider) GetFunctionMetadata() ([]*graph.Node, error) {
	query := `
		// Get Function Metadata
		MATCH (n:Function)
		WHERE coalesce(n.is_test, false) = false
		  AND NOT (size(n.atomic_features) = 1 AND n.atomic_features[0] = 'unknown')
		RETURN n.id as id, n.name as name, n.file as file, n.start_line as start_line, n.end_line as end_line, n.atomic_features as atomic_features
	`
	result, err := p.executeQuery(query, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to get function metadata: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		name, _, _ := neo4j.GetRecordValue[string](record, "name")
		file, _, _ := neo4j.GetRecordValue[string](record, "file")
		startLine, _, _ := neo4j.GetRecordValue[int64](record, "start_line")
		endLine, _, _ := neo4j.GetRecordValue[int64](record, "end_line")

		var atomicFeatures []string
		if val, found := record.Get("atomic_features"); found && val != nil {
			if rawAf, ok := val.([]any); ok {
				for _, v := range rawAf {
					if s, okStr := v.(string); okStr {
						atomicFeatures = append(atomicFeatures, s)
					}
				}
			}
		}

		nodes = append(nodes, &graph.Node{
			ID:    id,
			Label: "Function",
			Properties: map[string]any{
				"name":            name,
				"file":            file,
				"start_line":      startLine,
				"end_line":        endLine,
				"atomic_features": atomicFeatures,
			},
		})
	}
	return nodes, nil
}

// GetUnnamedFeatures returns features and domains without a generated name/description.
func (p *Neo4jProvider) GetUnnamedFeatures(limit int) ([]*graph.Node, error) {
	query := `
		// Get Unnamed Features
		MATCH (n)
		WHERE (n:Feature OR n:Domain) AND (coalesce(n.name, '') = '' OR coalesce(n.description, '') = '')
		RETURN n.id as id, labels(n)[0] as label, properties(n) as props
		LIMIT $limit
	`
	result, err := p.executeQuery(query, map[string]any{
		"limit": limit,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get unnamed features: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		label, _, _ := neo4j.GetRecordValue[string](record, "label")
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")

		nodes = append(nodes, &graph.Node{
			ID:         id,
			Label:      label,
			Properties: sanitizeProperties(props),
		})
	}
	return nodes, nil
}

// CountUnnamedFeatures returns the total number of features and domains without a name/description.
func (p *Neo4jProvider) CountUnnamedFeatures() (int64, error) {
	query := `
		// Count Unnamed Features
		MATCH (n)
		WHERE (n:Feature OR n:Domain) AND (coalesce(n.name, '') = '' OR coalesce(n.description, '') = '')
		RETURN count(n) as total
	`
	result, err := p.executeQuery(query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count unnamed features: %w", err)
	}
	if len(result.Records) == 0 {
		return 0, nil
	}
	total, _, _ := neo4j.GetRecordValue[int64](result.Records[0], "total")
	return total, nil
}

// ClearFeatureTopology deletes all Feature and Domain nodes.
func (p *Neo4jProvider) ClearFeatureTopology() error {
	query := `
		// Clear Feature Topology
		MATCH (n) WHERE n:Feature OR n:Domain DETACH DELETE n
	`
	_, err := p.executeQuery(query, nil)
	if err != nil {
		return fmt.Errorf("failed to clear feature topology: %w", err)
	}
	return nil
}

// UpdateFeatureTopology writes feature nodes and relationships to the graph
// using chunked batches to avoid monolithic transactions that can hang.
func (p *Neo4jProvider) UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	if len(nodes) == 0 && len(edges) == 0 {
		return nil
	}

	if err := p.batchWriteNodes(p.ctx, nodes, featureTopologyBatchSize); err != nil {
		return fmt.Errorf("failed to write feature nodes: %w", err)
	}

	if err := p.batchWriteEdges(p.ctx, edges, featureTopologyBatchSize); err != nil {
		return fmt.Errorf("failed to write feature edges: %w", err)
	}

	return nil
}

func (p *Neo4jProvider) batchWriteNodes(ctx context.Context, nodes []*graph.Node, batchSize int) error {
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

		nodeBatch := make([]map[string]any, 0, len(chunk))
		for _, n := range chunk {
			props := make(map[string]any)
			if n.Properties != nil {
				for k, v := range n.Properties {
					props[k] = v
				}
			}
			props["id"] = n.ID
			props["node_label"] = n.Label
			nodeBatch = append(nodeBatch, props)
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)

		session := p.driver.NewSession(batchCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
		_, err := session.ExecuteWrite(batchCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				UNWIND $batch AS row
				MERGE (n:CodeElement {id: row.id})
				SET n += row
				WITH n, row
				FOREACH (ignore IN CASE WHEN row.node_label = 'Domain' THEN [1] ELSE [] END | SET n:Domain)
				FOREACH (ignore IN CASE WHEN row.node_label = 'Feature' THEN [1] ELSE [] END | SET n:Feature)
			`
			logger.Query.Printf("Query: Batch Write Nodes (%d nodes)", len(nodeBatch))
			_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": nodeBatch})
			return nil, txErr
		})
		session.Close(batchCtx)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to write node batch %d/%d: %w", batchNum, totalBatches, err)
		}

		logger.Query.Printf("Writing feature topology: nodes batch %d/%d (%d/%d)", batchNum, totalBatches, end, len(nodes))
	}

	return nil
}

func (p *Neo4jProvider) batchWriteEdges(ctx context.Context, edges []*graph.Edge, batchSize int) error {
	if len(edges) == 0 {
		return nil
	}

	// Group edges by type
	edgeGroups := make(map[string][]*graph.Edge)
	for _, e := range edges {
		edgeGroups[e.Type] = append(edgeGroups[e.Type], e)
	}

	for relType, groupEdges := range edgeGroups {
		sanitizedRelType := loader.SanitizeLabel(relType)
		totalBatches := (len(groupEdges) + batchSize - 1) / batchSize

		for i := 0; i < len(groupEdges); i += batchSize {
			end := i + batchSize
			if end > len(groupEdges) {
				end = len(groupEdges)
			}
			chunk := groupEdges[i:end]
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
				`, sanitizedRelType)
				logger.Query.Printf("Query: Batch Write Edges [%s] (%d edges)", relType, len(batch))
				_, txErr := tx.Run(batchCtx, query, map[string]any{"batch": batch})
				return nil, txErr
			})
			session.Close(batchCtx)
			cancel()

			if err != nil {
				return fmt.Errorf("failed to write edge batch [%s] %d/%d: %w", relType, batchNum, totalBatches, err)
			}

			logger.Query.Printf("Writing feature topology: edges [%s] batch %d/%d (%d/%d)", relType, batchNum, totalBatches, end, len(groupEdges))
		}
	}

	return nil
}

// UpdateFeatureSummary saves the generated name and description for a feature or domain.
func (p *Neo4jProvider) UpdateFeatureSummary(id string, name string, description string) error {
	query := `
		// Update Feature Summary
		MATCH (n {id: $id})
		WHERE n:Feature OR n:Domain
		SET n.name = $name, n.description = $description
	`
	_, err := p.executeQuery(query, map[string]any{
		"id":          id,
		"name":        name,
		"description": description,
	})

	if err != nil {
		return fmt.Errorf("failed to update feature summary for %s: %w", id, err)
	}
	return nil
}

// CreateBatchJobNode creates or merges a :BatchJob node in Neo4j.
func (p *Neo4jProvider) CreateBatchJobNode(ctx context.Context, jobID, modelName, gcsInput, gcsOutput string) error {
	query := `
		MERGE (j:BatchJob {jobID: $jobID})
		SET j.modelName = $modelName,
		    j.gcsInputURI = $gcsInput,
		    j.gcsOutputURI = $gcsOutput,
		    j.state = "pending",
		    j.createdAt = coalesce(j.createdAt, $now),
		    j.updatedAt = $now
	`
	now := time.Now().UTC()
	_, err := neo4j.ExecuteQuery(ctx, p.driver, query, map[string]any{
		"jobID":     jobID,
		"modelName": modelName,
		"gcsInput":  gcsInput,
		"gcsOutput": gcsOutput,
		"now":       now,
	}, neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("failed to create batch job node %s: %w", jobID, err)
	}
	return nil
}

// UpdateBatchJobNodeStatus updates the status and failure reason of a :BatchJob node.
func (p *Neo4jProvider) UpdateBatchJobNodeStatus(ctx context.Context, jobID, state, failureReason string) error {
	query := `
		MATCH (j:BatchJob {jobID: $jobID})
		SET j.state = $state,
		    j.failureReason = $failureReason,
		    j.updatedAt = $now
	`
	now := time.Now().UTC()
	_, err := neo4j.ExecuteQuery(ctx, p.driver, query, map[string]any{
		"jobID":         jobID,
		"state":         state,
		"failureReason": failureReason,
		"now":           now,
	}, neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("failed to update batch job status %s: %w", jobID, err)
	}
	return nil
}

// GetActiveBatchJobs returns all active :BatchJob nodes.
func (p *Neo4jProvider) GetActiveBatchJobs(ctx context.Context) ([]BatchJob, error) {
	query := `
		MATCH (j:BatchJob)
		WHERE NOT j.state IN ['succeeded', 'failed', 'JOB_STATE_SUCCEEDED', 'JOB_STATE_FAILED', 'JOB_STATE_CANCELLED', 'JOB_STATE_EXPIRED']
		RETURN j.jobID as jobID,
		       j.state as state,
		       j.modelName as modelName,
		       j.gcsInputURI as gcsInputURI,
		       j.gcsOutputURI as gcsOutputURI,
		       j.failureReason as failureReason,
		       j.createdAt as createdAt,
		       j.updatedAt as updatedAt
	`
	result, err := neo4j.ExecuteQuery(ctx, p.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("failed to get active batch jobs: %w", err)
	}

	jobs := make([]BatchJob, 0, len(result.Records))
	for _, record := range result.Records {
		jobID, _, _ := neo4j.GetRecordValue[string](record, "jobID")
		state, _, _ := neo4j.GetRecordValue[string](record, "state")
		modelName, _, _ := neo4j.GetRecordValue[string](record, "modelName")
		gcsInputURI, _, _ := neo4j.GetRecordValue[string](record, "gcsInputURI")
		gcsOutputURI, _, _ := neo4j.GetRecordValue[string](record, "gcsOutputURI")
		failureReason, _, _ := neo4j.GetRecordValue[string](record, "failureReason")

		createdAtRaw, _ := record.Get("createdAt")
		createdAt, err := parseBatchJobTime(createdAtRaw)
		if err != nil {
			log.Printf("Warning: failed to parse createdAt for job %s: %v", jobID, err)
			createdAt = time.Time{}
		}

		updatedAtRaw, _ := record.Get("updatedAt")
		updatedAt, err := parseBatchJobTime(updatedAtRaw)
		if err != nil {
			log.Printf("Warning: failed to parse updatedAt for job %s: %v", jobID, err)
			updatedAt = time.Time{}
		}

		jobs = append(jobs, BatchJob{
			JobID:         jobID,
			State:         state,
			ModelName:     modelName,
			GCSInputURI:   gcsInputURI,
			GCSOutputURI:  gcsOutputURI,
			FailureReason: failureReason,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		})
	}
	return jobs, nil
}

// GetBatchJobCount returns the total number of BatchJobs (active or completed/failed).
func (p *Neo4jProvider) GetBatchJobCount(ctx context.Context) (int, error) {
	query := `
		MATCH (j:BatchJob)
		RETURN count(j) as count
	`
	result, err := neo4j.ExecuteQuery(ctx, p.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return 0, fmt.Errorf("failed to count batch jobs: %w", err)
	}
	if len(result.Records) == 0 {
		return 0, nil
	}
	count, _, _ := neo4j.GetRecordValue[int64](result.Records[0], "count")
	return int(count), nil
}

func parseBatchJobTime(val any) (time.Time, error) {
	if val == nil {
		return time.Time{}, nil
	}
	switch t := val.(type) {
	case time.Time:
		return t, nil
	case dbtype.Time:
		return t.Time(), nil
	case dbtype.LocalDateTime:
		return t.Time(), nil
	case string:
		return time.Parse(time.RFC3339, t)
	default:
		return time.Time{}, fmt.Errorf("unexpected date type: %T", val)
	}
}
