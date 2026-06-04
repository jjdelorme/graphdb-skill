package query

import (
	"context"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/graph"
	"graphdb/internal/logger"
	"graphdb/internal/progress"
	"graphdb/internal/tools/snippet"
	"path/filepath"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

// SetProgressActive is no longer used, as progress is tracked in progress.IsAnyActive()
func SetProgressActive(active bool) {}

func isProgressActive() bool {
	return progress.IsAnyActive()
}

// Neo4jProvider implements GraphProvider using the official Neo4j Go driver.
type Neo4jProvider struct {
	driver  neo4j.DriverWithContext
	ctx     context.Context
	dbName  string
	baseDir string
}

// NewNeo4jProvider creates a new connection to Neo4j.
func NewNeo4jProvider(cfg config.Config) (*Neo4jProvider, error) {
	auth := neo4j.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, "")

	driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI, auth, func(c *neo4j.Config) {
		c.MaxTransactionRetryTime = 30 * time.Second
		c.SocketKeepalive = true
		c.ConnectionLivenessCheckTimeout = 1 * time.Minute
	})
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
		driver:  driver,
		ctx:     ctx,
		dbName:  cfg.Neo4jDatabase,
		baseDir: cfg.BaseDir,
	}, nil
}

// Close closes the Neo4j driver connection.
func (p *Neo4jProvider) Close() error {
	return p.driver.Close(p.ctx)
}

// executeQuery logs the Cypher query and executes it via the driver.
func (p *Neo4jProvider) executeQuery(query string, params map[string]any) (*neo4j.EagerResult, error) {
	// Extract description from first line comment if it exists
	lines := strings.SplitN(strings.TrimSpace(query), "\n", 2)
	description := "Neo4j Query"
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "//") {
		description = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "//"))
	}

	// Use the dedicated Query logger, which only writes to the log file if enabled
	// and is always silent in the terminal.
	logger.Query.Printf("Query: %s", description)

	if params != nil {
		// Sanitize params for logging to avoid bloat (redact large vectors/embeddings)
		sanitized := make(map[string]any)
		for k, v := range params {
			if k == "embedding" || k == "v1" || k == "v2" {
				sanitized[k] = "[redacted/large vector]"
			} else {
				sanitized[k] = v
			}
		}
		logger.Query.Printf("Params: %v", sanitized)
	}
	return neo4j.ExecuteQuery(p.ctx, p.driver, query, params, neo4j.EagerResultTransformer, neo4j.ExecuteQueryWithDatabase(p.dbName))
}

// Traverse traverses the graph from a start node.
func (p *Neo4jProvider) Traverse(startNodeID string, relationship string, direction Direction, depth int) ([]*graph.Path, error) {
	// 1. Format relationships for Cypher (e.g., "CALLS,USES" -> "CALLS|USES")
	relPattern := ""
	if relationship != "" {
		relPattern = ":" + strings.ReplaceAll(relationship, ",", "|")
	}

	// 2. Determine arrow syntax based on direction
	arrowStart := "-"
	arrowEnd := "->"
	switch direction {
	case Incoming:
		arrowStart = "<-"
		arrowEnd = "-"
	case Both:
		arrowStart = "-"
		arrowEnd = "-"
	}

	// 3. Construct Cypher query
	query := fmt.Sprintf(`
		// Traverse Graph
		MATCH (n) WHERE n.id = $id OR n.fqn = $id OR n.name = $id
		MATCH p = (n)%s[%s*1..%d]%s(m)
		RETURN p
	`, arrowStart, relPattern, depth, arrowEnd)

	result, err := p.executeQuery(query, map[string]any{
		"id": startNodeID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute Traverse query: %w", err)
	}

	paths := make([]*graph.Path, 0, len(result.Records))
	for _, record := range result.Records {
		rawPath, _, err := neo4j.GetRecordValue[neo4j.Path](record, "p")
		if err != nil {
			continue
		}

		// Convert neo4j.Path to graph.Path
		gPath := &graph.Path{
			Nodes: make([]*graph.Node, len(rawPath.Nodes)),
			Edges: make([]*graph.Edge, len(rawPath.Relationships)),
		}

		for i, n := range rawPath.Nodes {
			label := ""
			if len(n.Labels) > 0 {
				label = n.Labels[0]
			}

			id := ""
			if idVal, ok := n.Props["id"].(string); ok {
				id = idVal
			} else if nameVal, ok := n.Props["name"].(string); ok {
				id = nameVal
			}

			gPath.Nodes[i] = &graph.Node{
				ID:         id,
				Label:      label,
				Properties: sanitizeProperties(n.Props),
			}
		}

		for i, r := range rawPath.Relationships {
			var sourceID, targetID string
			for _, n := range rawPath.Nodes {
				if n.ElementId == r.StartElementId {
					if idVal, ok := n.Props["id"].(string); ok {
						sourceID = idVal
					} else if nameVal, ok := n.Props["name"].(string); ok {
						sourceID = nameVal
					}
				}
				if n.ElementId == r.EndElementId {
					if idVal, ok := n.Props["id"].(string); ok {
						targetID = idVal
					} else if nameVal, ok := n.Props["name"].(string); ok {
						targetID = nameVal
					}
				}
			}

			gPath.Edges[i] = &graph.Edge{
				SourceID: sourceID,
				TargetID: targetID,
				Type:     r.Type,
			}
		}

		paths = append(paths, gPath)
	}

	return paths, nil
}

// SemanticTrace executes a targeted hierarchical query from Domain down to File.
func (p *Neo4jProvider) SemanticTrace(nodeID string) ([]*graph.Path, error) {
	// targeted hierarchy: [Domain] --(PARENT_OF)--> [Feature] --(IMPLEMENTS)--> [Function] --(DEFINED_IN)--> [File]
	query := `
		// Semantic Trace
		MATCH (func:Function) WHERE func.id = $targetId OR func.fqn = $targetId OR func.name = $targetId
		MATCH path = (d:Domain)-[:PARENT_OF*0..1]->(feat:Feature)-[:IMPLEMENTS*0..1]->(func)-[:DEFINED_IN*0..1]->(file:File)
		RETURN path
	`

	result, err := p.executeQuery(query, map[string]any{
		"targetId": nodeID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute SemanticTrace query: %w", err)
	}

	paths := make([]*graph.Path, 0, len(result.Records))
	for _, record := range result.Records {
		rawPath, _, err := neo4j.GetRecordValue[neo4j.Path](record, "path")
		if err != nil {
			continue
		}

		gPath := &graph.Path{
			Nodes: make([]*graph.Node, len(rawPath.Nodes)),
			Edges: make([]*graph.Edge, len(rawPath.Relationships)),
		}

		for i, n := range rawPath.Nodes {
			label := ""
			if len(n.Labels) > 0 {
				label = n.Labels[0]
			}
			id := ""
			if idVal, ok := n.Props["id"].(string); ok {
				id = idVal
			} else if nameVal, ok := n.Props["name"].(string); ok {
				id = nameVal
			}
			gPath.Nodes[i] = &graph.Node{
				ID:         id,
				Label:      label,
				Properties: sanitizeProperties(n.Props),
			}
		}

		for i, r := range rawPath.Relationships {
			sourceID := ""
			targetID := ""
			for _, n := range rawPath.Nodes {
				if n.ElementId == r.StartElementId {
					if idVal, ok := n.Props["id"].(string); ok {
						sourceID = idVal
					} else if nameVal, ok := n.Props["name"].(string); ok {
						sourceID = nameVal
					}
				}
				if n.ElementId == r.EndElementId {
					if idVal, ok := n.Props["id"].(string); ok {
						targetID = idVal
					} else if nameVal, ok := n.Props["name"].(string); ok {
						targetID = nameVal
					}
				}
			}
			gPath.Edges[i] = &graph.Edge{
				SourceID: sourceID,
				TargetID: targetID,
				Type:     r.Type,
			}
		}
		paths = append(paths, gPath)
	}

	return paths, nil
}

// SearchSimilarFunctions searches for function nodes using vector embeddings with a literal fallback.
func (p *Neo4jProvider) SearchSimilarFunctions(queryStr string, embedding []float32, limit int) ([]*FeatureResult, error) {
	// Check if embedding is valid (not all zeros)
	hasEmbedding := false
	for _, v := range embedding {
		if v != 0 {
			hasEmbedding = true
			break
		}
	}

	var query string
	params := map[string]any{
		"query": queryStr,
		"limit": limit,
	}

	if hasEmbedding {
		query = `
			// Search Similar Functions (with literal fallback)
			OPTIONAL MATCH (exact:Function)
			WHERE exact.name = $query OR exact.id = $query OR exact.fqn = $query OR exact.name STARTS WITH $query
			WITH collect(exact) as exactNodes

			CALL db.index.vector.queryNodes('function_embeddings', $limit, $embedding)
			YIELD node, score
			WITH exactNodes, node, score

			UNWIND (exactNodes + [node]) as n
			WITH n, max(CASE WHEN n IN exactNodes THEN 1.0 ELSE score END) as finalScore
			WHERE n IS NOT NULL
			RETURN n.id as id, labels(n)[0] as label, finalScore as score, properties(n) as props
			ORDER BY score DESC
			LIMIT $limit
		`
		params["embedding"] = embedding
	} else {
		query = `
			// Search Similar Functions (literal only fallback)
			MATCH (n:Function)
			WHERE n.name = $query OR n.id = $query OR n.fqn = $query OR n.name STARTS WITH $query
			RETURN n.id as id, labels(n)[0] as label, 1.0 as score, properties(n) as props
			ORDER BY score DESC
			LIMIT $limit
		`
	}

	result, err := p.executeQuery(query, params)

	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search on functions: %w", err)
	}

	features := make([]*FeatureResult, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, err := neo4j.GetRecordValue[string](record, "id")
		if err != nil {
			continue
		}
		label, _, _ := neo4j.GetRecordValue[string](record, "label")
		score, _, _ := neo4j.GetRecordValue[float64](record, "score")
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")
		propsMap := sanitizeProperties(props)

		// Reconstruct node
		node := &graph.Node{
			ID:         id,
			Label:      label,
			Properties: propsMap,
		}

		features = append(features, &FeatureResult{
			Node:  node,
			Score: float32(score),
		})
	}

	return features, nil
}

// SearchFeatures searches for Feature nodes using vector embeddings with a literal fallback.
func (p *Neo4jProvider) SearchFeatures(queryStr string, embedding []float32, limit int) ([]*FeatureResult, error) {
	// Check if embedding is valid (not all zeros)
	hasEmbedding := false
	for _, v := range embedding {
		if v != 0 {
			hasEmbedding = true
			break
		}
	}

	var query string
	params := map[string]any{
		"query": queryStr,
		"limit": limit,
	}

	if hasEmbedding {
		query = `
			// Search Features (with literal fallback)
			OPTIONAL MATCH (exact:Feature)
			WHERE exact.name = $query OR exact.id = $query OR exact.name CONTAINS $query
			WITH collect(exact) as exactNodes

			CALL db.index.vector.queryNodes('feature_embeddings', $limit, $embedding)
			YIELD node, score
			WITH exactNodes, node, score

			UNWIND (exactNodes + [node]) as n
			WITH n, max(CASE WHEN n IN exactNodes THEN 1.0 ELSE score END) as finalScore
			WHERE n IS NOT NULL
			RETURN n.id as id, labels(n)[0] as label, finalScore as score, properties(n) as props
			ORDER BY score DESC
			LIMIT $limit
		`
		params["embedding"] = embedding
	} else {
		query = `
			// Search Features (literal only fallback)
			MATCH (n:Feature)
			WHERE n.name = $query OR n.id = $query OR n.name CONTAINS $query
			RETURN n.id as id, labels(n)[0] as label, 1.0 as score, properties(n) as props
			ORDER BY score DESC
			LIMIT $limit
		`
	}

	result, err := p.executeQuery(query, params)

	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search on features: %w", err)
	}

	features := make([]*FeatureResult, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, err := neo4j.GetRecordValue[string](record, "id")
		if err != nil {
			continue
		}
		score, _, _ := neo4j.GetRecordValue[float64](record, "score")
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")

		node := &graph.Node{
			ID:         id,
			Label:      "Feature",
			Properties: sanitizeProperties(props),
		}

		features = append(features, &FeatureResult{
			Node:  node,
			Score: float32(score),
		})
	}

	return features, nil
}

// GetNeighbors retrieves the dependencies (functions, globals) of a node.
func (p *Neo4jProvider) GetNeighbors(nodeID string, depth int, limit int) (*NeighborResult, error) {
	query := fmt.Sprintf(`
		// Get Neighbors
		MATCH (n)
		WHERE n.id = $func OR n.fqn = $func OR n.name = $func
		WITH n LIMIT 1
		
		// Expand scope if n is a Class (include its methods)
		OPTIONAL MATCH (n)-[:HAS_METHOD]->(m)
		WITH n, collect(m) + n as scope
		UNWIND scope as s

		// 1. Direct & Transitive Globals
		OPTIONAL MATCH path = (s)-[:CALLS*0..%d]->(callee)-[:USES_GLOBAL]->(g:Global)
		WITH n, collect(DISTINCT CASE WHEN g IS NOT NULL THEN {
			dependency: g.name, 
			type: 'Global', 
			via: [x in nodes(path) WHERE x.id <> s.id | x.name]
		} ELSE NULL END) as globals

		// 2. Direct Function Calls / Uses
		OPTIONAL MATCH (s)-[:CALLS|USES]->(d)
		WITH n, globals, collect(DISTINCT CASE WHEN d IS NOT NULL THEN {dependency: d.name, type: head(labels(d)), labels: labels(d)} ELSE NULL END) as funcs

		RETURN n, (globals + funcs)[0..$limit] as dependencies
		`, depth)

		result, err := p.executeQuery(query, map[string]any{
		"func":  nodeID,
		"limit": limit,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to execute GetNeighbors query: %w", err)
	}

	if len(result.Records) == 0 {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	dependenciesRaw, _, err := neo4j.GetRecordValue[[]any](result.Records[0], "dependencies")
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies from record: %w", err)
	}

	// Fetch the full node properties for the result
	var targetNode *graph.Node
	if n, ok := result.Records[0].Get("n"); ok {
		if neoNode, ok := n.(neo4j.Node); ok {
			id := ""
			if idVal, ok := neoNode.Props["id"].(string); ok {
				id = idVal
			} else if nameVal, ok := neoNode.Props["name"].(string); ok {
				id = nameVal
			}
			label := ""
			if len(neoNode.Labels) > 0 {
				label = neoNode.Labels[0]
			}
			targetNode = &graph.Node{
				ID:         id,
				Label:      label,
				Properties: sanitizeProperties(neoNode.Props),
			}
		}
	}

	if targetNode == nil {
		targetNode = &graph.Node{ID: nodeID, Label: "Unknown"}
	}

	deps := make([]Dependency, 0, len(dependenciesRaw))
	for _, raw := range dependenciesRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		name, _ := item["dependency"].(string)
		typ, _ := item["type"].(string)

		dep := Dependency{
			Name: name,
			Type: typ,
		}

		if viaRaw, ok := item["via"]; ok && viaRaw != nil {
			if viaList, ok := viaRaw.([]any); ok {
				via := make([]string, len(viaList))
				for i, v := range viaList {
					if s, ok := v.(string); ok {
						via[i] = s
					}
				}
				dep.Via = via
			}
		}
		deps = append(deps, dep)
	}

	return &NeighborResult{
		Node:         targetNode,
		Dependencies: deps,
	}, nil
}

// GetCallers retrieves the callers of a node.
func (p *Neo4jProvider) GetCallers(nodeID string) ([]string, error) {
	query := `
		// Get Callers
		MATCH (n) WHERE n.id = $func OR n.fqn = $func OR n.name = $func
		MATCH (caller)-[:CALLS]->(n)
		RETURN collect(DISTINCT caller.name) as callers
	`

	result, err := p.executeQuery(query, map[string]any{
		"func": nodeID,
	})

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
		if s, ok := raw.(string); ok {
			callers[i] = s
		}
	}

	return callers, nil
}

// GetImpact analyzes the impact of changing a node (reverse dependencies).
func (p *Neo4jProvider) GetImpact(nodeID string, depth int) (*ImpactResult, error) {
	// Construct dynamic query for variable path length
	query := fmt.Sprintf(`
		// Get Impact
		MATCH (n) WHERE n.id = $nodeID OR n.fqn = $nodeID OR n.name = $nodeID
		MATCH (caller)-[:CALLS*1..%d]->(n) 
		RETURN DISTINCT caller.name as caller, caller.is_volatile as volatile
	`, depth)

	result, err := p.executeQuery(query, map[string]any{
		"nodeID": nodeID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetImpact query: %w", err)
	}

	callers := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		label, _, err := neo4j.GetRecordValue[string](record, "caller")
		if err != nil {
			continue
		}
		volatile, _, _ := neo4j.GetRecordValue[bool](record, "volatile")

		node := &graph.Node{
			Label: label,
			Properties: map[string]any{
				"is_volatile": volatile,
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
		// Get Globals
		MATCH (n) WHERE n.id = $nodeID OR n.fqn = $nodeID OR n.name = $nodeID
		MATCH (n)-[:USES_GLOBAL]->(g:Global) 
		RETURN g.name as name, g.file as defined_in
	`

	result, err := p.executeQuery(query, map[string]any{
		"nodeID": nodeID,
	})

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

// GetSeams suggests architectural seams (boundaries) using Pinch Point detection.
func (p *Neo4jProvider) GetSeams(modulePattern string, layer string) ([]*SeamResult, error) {
	// Pre-flight check: Is is_volatile data present?
	checkQuery := `
		// Check Volatility Data Presence
		MATCH (f:Function) WHERE f.is_volatile IS NOT NULL RETURN count(f) AS count LIMIT 1
	`
	checkRes, err := p.executeQuery(checkQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("pre-flight check failed: %w", err)
	}
	if len(checkRes.Records) == 0 {
		return nil, fmt.Errorf("volatility data is missing. Run 'graphdb enrich-contamination' first")
	}
	count, _, _ := neo4j.GetRecordValue[int64](checkRes.Records[0], "count")
	if count == 0 {
		return nil, fmt.Errorf("volatility data is missing. Run 'graphdb enrich-contamination' first")
	}

	// Pinch Point detection:
	// Find functions that have high internal fan-in (non-volatile callers)
	// and high volatile fan-out (volatile callees).
	// This represents a 'pinch point' or 'chokepoint' between internal and external worlds.
	query := `
		// Find Pinch Points
		MATCH (f:Function)
		// Filter by module pattern if provided
		OPTIONAL MATCH (f)-[:DEFINED_IN]->(file:File)
		WHERE ($pattern = "" OR file.file =~ $pattern)
		
		// Internal Fan-In: Non-volatile callers
		OPTIONAL MATCH (caller:Function)-[:CALLS]->(f)
		WHERE (caller.is_volatile = false OR caller.is_volatile IS NULL)
		WITH f, count(DISTINCT caller) AS internal_fan_in, file

		// Volatile Fan-Out: Volatile callees
		OPTIONAL MATCH (f)-[:CALLS]->(callee:Function)
		WHERE callee.is_volatile = true
		WITH f, internal_fan_in, count(DISTINCT callee) AS volatile_fan_out, file

		WHERE internal_fan_in > 0 AND volatile_fan_out > 0
		RETURN f.name as seam, file.file as file, (internal_fan_in * volatile_fan_out) as risk, "pinch-point" as type
		ORDER BY risk DESC
		LIMIT 20
	`

	result, err := p.executeQuery(query, map[string]any{
		"pattern": modulePattern,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetSeams (Pinch Point) query: %w", err)
	}

	seams := make([]*SeamResult, 0, len(result.Records))
	for _, record := range result.Records {
		seam, _, err := neo4j.GetRecordValue[string](record, "seam")
		if err != nil {
			continue
		}
		file, _, _ := neo4j.GetRecordValue[string](record, "file")
		seamType, _, _ := neo4j.GetRecordValue[string](record, "type")

		var risk float64
		// risk might be int64 or float64 depending on Cypher result
		if riskVal, ok := record.Get("risk"); ok && riskVal != nil {
			switch v := riskVal.(type) {
			case float64:
				risk = v
			case int64:
				risk = float64(v)
			case int:
				risk = float64(v)
			}
		}

		seams = append(seams, &SeamResult{
			Seam: seam,
			File: file,
			Risk: risk,
			Type: seamType,
		})
	}

	return seams, nil
}

// FetchSource retrieves the source code for a node.
func (p *Neo4jProvider) FetchSource(nodeID string) (string, error) {
	query := `
		// Fetch Source
		MATCH (n) WHERE n.id = $id OR n.fqn = $id OR n.name = $id
		RETURN n.file as file, n.start_line as start, n.end_line as end
	`
	result, err := p.executeQuery(query, map[string]any{
		"id": nodeID,
	})

	if err != nil {
		return "", fmt.Errorf("failed to query source info: %w", err)
	}

	if len(result.Records) == 0 {
		return "", fmt.Errorf("node not found: %s", nodeID)
	}

	record := result.Records[0]
	file, _, _ := neo4j.GetRecordValue[string](record, "file")
	start, _, _ := neo4j.GetRecordValue[int64](record, "start")
	end, _, _ := neo4j.GetRecordValue[int64](record, "end")

	if file == "" {
		return "", fmt.Errorf("node %s has no file associated", nodeID)
	}

	if start == 0 && end == 0 {
		// Default to first 50 lines if no line info
		start = 1
		end = 50
	}

	fullPath := file
	if p.baseDir != "" && p.baseDir != "." {
		fullPath = filepath.Join(p.baseDir, file)
	}

	return snippet.SliceFile(fullPath, int(start), int(end))
}

// LocateUsage identifies where a dependency is used within a function.
func (p *Neo4jProvider) LocateUsage(sourceID string, targetID string) (any, error) {
	query := `
		// Locate Usage
		MATCH (source) WHERE source.id = $sourceId OR source.fqn = $sourceId OR source.name = $sourceId
		MATCH (target) WHERE target.id = $targetId OR target.fqn = $targetId OR target.name = $targetId
		RETURN source.file as file, source.start_line as start, source.end_line as end, target.name as target_name, properties(target).name as target_name_alt
	`
	result, err := p.executeQuery(query, map[string]any{
		"sourceId": sourceID,
		"targetId": targetID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query usage info: %w", err)
	}

	if len(result.Records) == 0 {
		return nil, fmt.Errorf("source or target node not found")
	}

	record := result.Records[0]
	file, _, _ := neo4j.GetRecordValue[string](record, "file")
	start, _, _ := neo4j.GetRecordValue[int64](record, "start")
	end, _, _ := neo4j.GetRecordValue[int64](record, "end")
	targetName, _, _ := neo4j.GetRecordValue[string](record, "target_name")
	if targetName == "" {
		targetName, _, _ = neo4j.GetRecordValue[string](record, "target_name_alt")
	}

	if file == "" || start == 0 || end == 0 {
		return nil, fmt.Errorf("source node %s missing location info", sourceID)
	}

	fullPath := file
	if p.baseDir != "" && p.baseDir != "." {
		fullPath = filepath.Join(p.baseDir, file)
	}

	content, err := snippet.SliceFile(fullPath, int(start), int(end))
	if err != nil {
		return nil, err
	}

	return snippet.FindPatternInScope(content, targetName, 0, int(start))
}

// GetOverview returns a high-level graph of top-level semantic nodes.
func (p *Neo4jProvider) GetOverview() (*graph.Path, error) {
	query := `
		// Get Overview
		MATCH (n) WHERE n:Domain OR (n:Feature AND NOT ()-[]->(n)) 
		RETURN n, null as p
	`
	result, err := p.executeQuery(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GetOverview query: %w", err)
	}

	outPath := &graph.Path{
		Nodes: make([]*graph.Node, 0),
		Edges: make([]*graph.Edge, 0),
	}

	nodeMap := make(map[string]bool)
	edgeMap := make(map[string]bool)

	for _, record := range result.Records {
		// Single isolated nodes
		if rawN, ok, _ := neo4j.GetRecordValue[neo4j.Node](record, "n"); ok {
			id := rawN.ElementId
			if idProp, hasId := rawN.Props["id"].(string); hasId {
				id = idProp
			}

			label := "Unknown"
			if len(rawN.Labels) > 0 {
				label = rawN.Labels[0]
			}

			if !nodeMap[id] {
				nodeMap[id] = true
				outPath.Nodes = append(outPath.Nodes, &graph.Node{
					ID:         id,
					Label:      label,
					Properties: sanitizeProperties(rawN.Props),
				})
			}
		} else if rawAny, ok := record.Get("n"); ok {
			var id, label string
			var props map[string]any

			if node, ok := rawAny.(neo4j.Node); ok {
				id = node.ElementId
				if idProp, hasId := node.Props["id"].(string); hasId {
					id = idProp
				}
				if len(node.Labels) > 0 {
					label = node.Labels[0]
				}
				props = node.Props
			} else if dbnode, ok := rawAny.(dbtype.Node); ok {
				id = dbnode.ElementId
				if idProp, hasId := dbnode.Props["id"].(string); hasId {
					id = idProp
				}
				if len(dbnode.Labels) > 0 {
					label = dbnode.Labels[0]
				}
				props = dbnode.Props
			}

			if id != "" && !nodeMap[id] {
				nodeMap[id] = true
				if label == "" {
					label = "Unknown"
				}
				outPath.Nodes = append(outPath.Nodes, &graph.Node{
					ID:         id,
					Label:      label,
					Properties: sanitizeProperties(props),
				})
			}
		}

		// Paths (if they exist)
		if rawP, ok, _ := neo4j.GetRecordValue[neo4j.Path](record, "p"); ok {
			for _, n := range rawP.Nodes {
				id := n.ElementId
				if idProp, hasId := n.Props["id"].(string); hasId {
					id = idProp
				}

				label := "Unknown"
				if len(n.Labels) > 0 {
					label = n.Labels[0]
				}

				if !nodeMap[id] {
					nodeMap[id] = true
					outPath.Nodes = append(outPath.Nodes, &graph.Node{
						ID:         id,
						Label:      label,
						Properties: sanitizeProperties(n.Props),
					})
				}
			}
			for _, r := range rawP.Relationships {
				// We need string IDs for source and target which match the Node IDs
				// Path in Neo4j-Go driver doesn't give us the string ID directly for endpoints unless we look them up in the path nodes

				sourceId := ""
				targetId := ""
				for _, n := range rawP.Nodes {
					if n.ElementId == r.StartElementId {
						if p, has := n.Props["id"].(string); has {
							sourceId = p
						} else {
							sourceId = n.ElementId
						}
					}
					if n.ElementId == r.EndElementId {
						if p, has := n.Props["id"].(string); has {
							targetId = p
						} else {
							targetId = n.ElementId
						}
					}
				}

				edgeKey := fmt.Sprintf("%s-%s-%s", sourceId, targetId, r.Type)
				if !edgeMap[edgeKey] && sourceId != "" && targetId != "" {
					edgeMap[edgeKey] = true
					outPath.Edges = append(outPath.Edges, &graph.Edge{
						SourceID: sourceId,
						TargetID: targetId,
						Type:     r.Type,
					})
				}
			}
		}
	}
	return outPath, nil
}

// ExploreDomain returns the hierarchy context for a Feature node:
// the feature itself, its parent, children, siblings, and implementing functions.
func (p *Neo4jProvider) ExploreDomain(featureID string) (*DomainExplorationResult, error) {
	query := `
		// Explore Domain
		// Find the target feature or domain
		MATCH (f {id: $featureID})
		WHERE f:Feature OR f:Domain

		// Optional: parent domain/feature
		OPTIONAL MATCH (parent)-[:PARENT_OF]->(f)
		WHERE parent:Feature OR parent:Domain

		// Optional: children
		OPTIONAL MATCH (f)-[:PARENT_OF]->(child)
		WHERE child:Feature OR child:Domain

		// Optional: siblings (same parent, different node)
		OPTIONAL MATCH (parent)-[:PARENT_OF]->(sibling)
		WHERE (sibling:Feature OR sibling:Domain) AND sibling.id <> f.id

		// Optional: implementing functions (via self or descendants)
		OPTIONAL MATCH (f)-[:PARENT_OF*0..]->(desc)
		OPTIONAL MATCH (fn:Function)-[:IMPLEMENTS]->(desc)

		RETURN properties(f) as feature, f.id as fid,
		       properties(parent) as parent, parent.id as pid,
		       collect(DISTINCT {id: child.id, props: properties(child)}) as children,
		       collect(DISTINCT {id: sibling.id, props: properties(sibling)}) as siblings,
		       collect(DISTINCT {id: fn.id, props: properties(fn)}) as functions
	`

	result, err := p.executeQuery(query, map[string]any{
		"featureID": featureID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute ExploreDomain query: %w", err)
	}

	if len(result.Records) == 0 {
		return nil, fmt.Errorf("feature not found: %s", featureID)
	}

	record := result.Records[0]

	// Build feature node
	fid, _, _ := neo4j.GetRecordValue[string](record, "fid")
	featureProps, _, _ := neo4j.GetRecordValue[map[string]any](record, "feature")
	featureNode := &graph.Node{ID: fid, Label: "Feature", Properties: sanitizeProperties(featureProps)}

	// Build parent node
	var parentNode *graph.Node
	pid, _, _ := neo4j.GetRecordValue[string](record, "pid")
	if pid != "" {
		parentProps, _, _ := neo4j.GetRecordValue[map[string]any](record, "parent")
		parentNode = &graph.Node{ID: pid, Label: "Feature", Properties: sanitizeProperties(parentProps)}
	}

	// Helper to extract node list from collected results
	extractNodes := func(key string, label string) []*graph.Node {
		raw, _, _ := neo4j.GetRecordValue[[]any](record, key)
		var nodes []*graph.Node
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			if id == "" {
				continue
			}
			props, _ := m["props"].(map[string]any)
			nodes = append(nodes, &graph.Node{ID: id, Label: label, Properties: sanitizeProperties(props)})
		}
		return nodes
	}

	return &DomainExplorationResult{
		Feature:   featureNode,
		Parent:    parentNode,
		Children:  extractNodes("children", "Feature"),
		Siblings:  extractNodes("siblings", "Feature"),
		Functions: extractNodes("functions", "Function"),
	}, nil
}

// GetGraphState retrieves the stored commit hash from the graph.
func (p *Neo4jProvider) GetGraphState() (string, error) {
	// Query for both legacy GraphState and newer Metadata nodes (possibly per-directory).
	// If a directory is set, we prefer Metadata matching that directory.
	query := `
		// Get Graph State
		MATCH (s)
		WHERE s:GraphState OR (s:Metadata AND ($dir = "" OR s.dir = $dir))
		RETURN s.commit as commit, labels(s)[0] as label
		ORDER BY label DESC, s.updatedAt DESC
		LIMIT 1
	`
	result, err := p.executeQuery(query, map[string]any{
		"dir": p.baseDir,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query graph state: %w", err)
	}

	if len(result.Records) == 0 {
		return "", nil // No state stored
	}

	commit, _, err := neo4j.GetRecordValue[string](result.Records[0], "commit")
	if err != nil {
		return "", fmt.Errorf("failed to get commit from record: %w", err)
	}

	return commit, nil
}

// GetStats returns basic statistics about the graph like total nodes.
func (p *Neo4jProvider) GetStats() (map[string]int64, error) {
	stats := make(map[string]int64)

	query := `
		// Get Graph Stats
		MATCH (n) RETURN count(n) as total
	`
	result, err := p.executeQuery(query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get node count: %w", err)
	}

	if len(result.Records) > 0 {
		val, _ := result.Records[0].Get("total")
		if count, ok := val.(int64); ok {
			stats["nodes"] = count
		} else if countInt, ok := val.(int); ok {
			stats["nodes"] = int64(countInt)
		}
	}

	return stats, nil
}

// sanitizeProperties removes heavy fields like embeddings from node properties
// to prevent context flooding in CLI output.
func sanitizeProperties(props map[string]any) map[string]any {
	if props == nil {
		return nil
	}
	clean := make(map[string]any, len(props))
	for k, v := range props {
		// Filter out vector embeddings
		if k == "embedding" {
			continue
		}
		clean[k] = v
	}
	return clean
}

// WipeDatabase clears all data from the database using a batched transaction delete
// and falling back to a non-transactional delete if the batch query fails.
func (p *Neo4jProvider) WipeDatabase(ctx context.Context) error {
	session := p.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: p.dbName})
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
	logger.Query.Printf("Warning: batch transactional wipe failed: %v. Falling back to non-transactional wipe.", err)
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
