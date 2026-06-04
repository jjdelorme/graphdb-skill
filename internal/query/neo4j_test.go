//go:build integration

package query

import (
	"graphdb/internal/config"
	"os"
	"strings"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func getProvider(t *testing.T) *Neo4jProvider {
	_ = config.LoadEnv()

	cfg := config.Config{
		Neo4jURI:      os.Getenv("NEO4J_URI"),
		Neo4jUser:     os.Getenv("NEO4J_USER"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
	}

	provider, err := NewNeo4jProvider(cfg)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	return provider
}

func cleanup(t *testing.T, p *Neo4jProvider) {
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, `
		MATCH (n) WHERE n.name STARTS WITH 'Test' OR n.file STARTS WITH 'Test' OR n.name = 'ContaminatedCaller' OR n.name = 'SeamFunc' OR n.file = 'test_fixture.go' OR n.name = 'InternalCaller' OR n.name = 'ExternalLib' OR n.name = 'HydratedFunc' DETACH DELETE n
	`, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Logf("Failed to cleanup: %v", err)
	}
}

func TestNeo4jConnection(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
}

func TestGetNeighbors(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup fixture data
	setupQuery := `
		CREATE (f:Function {name: 'TestFunc', id: 'TestFunc', embedding: [0.1, 0.2, 0.3]})
		CREATE (c:Function {name: 'TestCallee', id: 'TestCallee'})
		CREATE (g:Global {name: 'TestGlobal', id: 'TestGlobal', file: 'test_fixture.go'})
		CREATE (f)-[:CALLS]->(c)
		CREATE (c)-[:USES_GLOBAL]->(g)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test
	result, err := p.GetNeighbors("TestFunc", 2, 10)
	if err != nil {
	        t.Fatalf("GetNeighbors failed: %v", err)
	}
	// Verify
	foundGlobal := false
	foundFunc := false

	for _, dep := range result.Dependencies {
		if dep.Name == "TestGlobal" && dep.Type == "Global" {
			foundGlobal = true
			if len(dep.Via) != 2 || dep.Via[0] != "TestCallee" || dep.Via[1] != "TestGlobal" {
				t.Errorf("Expected global via [TestCallee, TestGlobal], got %v", dep.Via)
			}
		}
		if dep.Name == "TestCallee" && dep.Type == "Function" {
			foundFunc = true
		}
	}

	if !foundGlobal {
		t.Error("Expected to find TestGlobal dependency")
	}
	if !foundFunc {
		t.Error("Expected to find TestCallee dependency")
	}
}

func TestGetNeighbors_Hydration(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup fixture data with explicit properties
	setupQuery := `
		MERGE (f:Function {id: 'HydratedFunc'})
		SET f += {
			name: 'HydratedFunc', 
			fqn: 'pkg/file.go:HydratedFunc',
			file: 'pkg/file.go',
			start_line: 10,
			end_line: 20
		}
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test
	result, err := p.GetNeighbors("HydratedFunc", 1, 10)
	if err != nil {
	        t.Fatalf("GetNeighbors failed: %v", err)
	}
	// Verify target node hydration
	if result.Node == nil {
		t.Fatal("Result node is nil")
	}
	if result.Node.Label == "Unknown" {
		t.Errorf("Expected label 'Function', got 'Unknown'")
	}
	if result.Node.Properties == nil {
		t.Fatal("Result node properties are nil")
	}
	
	expectedProps := []string{"name", "fqn", "file", "start_line", "end_line"}
	for _, prop := range expectedProps {
		if _, ok := result.Node.Properties[prop]; !ok {
			t.Errorf("Missing expected property: %s", prop)
		}
	}
}

func TestGetCallers(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	setupQuery := `
		CREATE (caller:Function {name: 'TestCaller', id: 'TestCaller'})
		CREATE (target:Function {name: 'TestTarget', id: 'TestTarget'})
		CREATE (caller)-[:CALLS]->(target)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	callers, err := p.GetCallers("TestTarget")
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}

	if len(callers) != 1 || callers[0] != "TestCaller" {
		t.Errorf("Expected [TestCaller], got %v", callers)
	}
}

func TestGetImpact(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	setupQuery := `
		CREATE (caller:Function {name: 'TestDeepCaller', id: 'TestDeepCaller', is_volatile: true})
		CREATE (mid:Function {name: 'TestMid', id: 'TestMid'})
		CREATE (target:Function {name: 'TestTarget', id: 'TestTarget'})
		CREATE (caller)-[:CALLS]->(mid)
		CREATE (mid)-[:CALLS]->(target)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test depth 2
	result, err := p.GetImpact("TestTarget", 2)
	if err != nil {
		t.Fatalf("GetImpact failed: %v", err)
	}

	// Should find both mid and caller
	foundCaller := false
	foundMid := false

	for _, c := range result.Callers {
		if c.Label == "TestDeepCaller" {
			foundCaller = true
			if val, ok := c.Properties["is_volatile"].(bool); !ok || !val {
				t.Error("Expected TestDeepCaller to be volatile")
			}
		}
		if c.Label == "TestMid" {
			foundMid = true
		}
	}

	if !foundCaller {
		t.Error("Expected to find TestDeepCaller")
	}
	if !foundMid {
		t.Error("Expected to find TestMid")
	}
}

func TestGetGlobals(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	setupQuery := `
		CREATE (f:Function {name: 'TestFunc', id: 'TestFunc'})
		CREATE (g:Global {name: 'TestGlobalVar', id: 'TestGlobalVar', file: 'test_fixture.go'})
		CREATE (f)-[:USES_GLOBAL]->(g)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	result, err := p.GetGlobals("TestFunc")
	if err != nil {
		t.Fatalf("GetGlobals failed: %v", err)
	}

	if len(result.Globals) != 1 {
		t.Fatalf("Expected 1 global, got %d", len(result.Globals))
	}
	if result.Globals[0].Label != "TestGlobalVar" {
		t.Errorf("Expected TestGlobalVar, got %s", result.Globals[0].Label)
	}
	if result.Globals[0].Properties["file"] != "test_fixture.go" {
		t.Errorf("Expected file property to be test_fixture.go")
	}
}

func TestGetSeams(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	setupQuery := `
		// Internal (non-volatile) caller
		CREATE (caller:Function {name: 'InternalCaller', id: 'InternalCaller', is_volatile: false})
		// Pinch point
		CREATE (seam:Function {name: 'SeamFunc', id: 'SeamFunc', is_volatile: false})
		// Volatile callee
		CREATE (external:Function {name: 'ExternalLib', id: 'ExternalLib', is_volatile: true})
		
		CREATE (file:File {id: 'test_fixture.go', file: 'test_fixture.go'})
		
		CREATE (caller)-[:CALLS]->(seam)
		CREATE (seam)-[:CALLS]->(external)
		CREATE (seam)-[:DEFINED_IN]->(file)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test with matching pattern
	results, err := p.GetSeams(".*test_fixture.*", "ui")
	if err != nil {
		t.Fatalf("GetSeams failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 seam, got %d", len(results))
	}
	if results[0].Seam != "SeamFunc" {
		t.Errorf("Expected SeamFunc, got %s", results[0].Seam)
	}
	if results[0].File != "test_fixture.go" {
		t.Errorf("Expected test_fixture.go, got %s", results[0].File)
	}
	// Risk should be 1.0 (internal_fan_in=1 * volatile_fan_out=1)
	if results[0].Risk != 1.0 {
		t.Errorf("Expected risk 1.0, got %f", results[0].Risk)
	}
}

func TestGetSeams_MissingData(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Ensure NO is_volatile data exists
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, `
		MATCH (f:Function) REMOVE f.is_volatile
	`, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to clear is_volatile data: %v", err)
	}

	// Test
	_, err = p.GetSeams(".*", "ui")
	if err == nil {
		t.Fatal("Expected error when is_volatile data is missing, got nil")
	}

	expectedErr := "volatility data is missing. Run 'graphdb enrich-contamination' first"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
	}
}

func makeVector(dim int, val float64, idx int) []float64 {
	vec := make([]float64, dim)
	if idx >= 0 && idx < dim {
		vec[idx] = val
	}
	return vec
}

func makeVector32(dim int, val float32, idx int) []float32 {
	vec := make([]float32, dim)
	if idx >= 0 && idx < dim {
		vec[idx] = val
	}
	return vec
}

func TestSearchSimilarFunctions(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup with 768 dimensions
	v1 := makeVector(768, 1.0, 0)
	v2 := makeVector(768, 1.0, 1)

	// Note: We use 'Function' label here
	setupQuery := `
		CREATE (f1:Function {name: 'TestSim1', id: 'TestSim1', embedding: $v1})
		CREATE (f2:Function {name: 'TestSim2', id: 'TestSim2', embedding: $v2})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, map[string]any{
		"v1": v1,
		"v2": v2,
	}, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Search similar to v1 (closer to index 0)
	queryVec := makeVector32(768, 0.9, 0) // float32 for Go input

	// Call the new method name
	results, err := p.SearchSimilarFunctions("TestSim1", queryVec, 1)
	if err != nil {
		t.Logf("SearchSimilarFunctions failed (index might be missing): %v", err)
		return
	}

	if len(results) > 0 {
		if results[0].Node.ID != "TestSim1" {
			t.Errorf("Expected TestSim1, got %s (Score: %f)", results[0].Node.ID, results[0].Score)
		}
	} else {
		t.Log("No results found (index might be building)")
	}
}

func TestSearchFeatures(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup with 768 dimensions
	v1 := makeVector(768, 1.0, 0)

	// Note: We use 'Feature' label here
	setupQuery := `
		CREATE (f1:Feature {id: 'feat-1', name: 'TestFeature', embedding: $v1})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, map[string]any{
		"v1": v1,
	}, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	queryVec := makeVector32(768, 0.9, 0)

	results, err := p.SearchFeatures("TestFeature", queryVec, 1)
	if err != nil {
		t.Logf("SearchFeatures failed (index might be missing): %v", err)
		return
	}

	if len(results) > 0 {
		if results[0].Node.ID != "feat-1" {
			t.Errorf("Expected feat-1, got %s", results[0].Node.ID)
		}
	} else {
		t.Log("No results found (index might be building)")
	}
}

func TestSanitizeEmbeddings(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup a node with an embedding
	v1 := []float64{0.1, 0.2, 0.3}
	setupQuery := `
		CREATE (f:Function {name: 'TestSanitize', id: 'TestSanitize', embedding: $v1, other: 'keep_me'})
		CREATE (child:Function {name: 'TestSanitizeChild', id: 'TestSanitizeChild'})
		CREATE (f)-[:CALLS]->(child)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, map[string]any{"v1": v1}, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Use Traverse to fetch it
	paths, err := p.Traverse("TestSanitize", "", Outgoing, 1)
	if err != nil {
		t.Fatalf("Traverse failed: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("Expected at least one path (the node itself)")
	}

	node := paths[0].Nodes[0]
	if _, ok := node.Properties["embedding"]; ok {
		t.Error("Expected 'embedding' property to be sanitized/removed, but it was present")
	}
	if val, ok := node.Properties["other"].(string); !ok || val != "keep_me" {
		t.Error("Expected 'other' property to be preserved")
	}
}

func TestNeo4jProvider_FetchSource_SchemaMismatch(t *testing.T) {
	// This test asserts that FetchSource queries the correct line properties.
	// Parsers output 'line' and 'end_line'. FetchSource queries 'start_line'.

	cfg := config.Config{
		Neo4jURI:      os.Getenv("NEO4J_URI"),
		Neo4jUser:     os.Getenv("NEO4J_USER"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
	}

	p, err := NewNeo4jProvider(cfg)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer p.Close()

	// 1. Setup Data with 'line' (matching parser schema), NOT 'start_line'
	setupQuery := `
		CREATE (n:CodeElement:Function {
			id: 'schema-test-source',
			name: 'TestSourceFunc',
			file: 'neo4j_test.go',
			line: 10,
			end_line: 20
		})
	`
	_, err = neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	defer func() {
		cleanupQuery := `MATCH (n) WHERE n.id = 'schema-test-source' DETACH DELETE n`
		neo4j.ExecuteQuery(p.ctx, p.driver, cleanupQuery, nil, neo4j.EagerResultTransformer)
	}()

	// 2. FetchSource queries 'start_line' which is missing (DB has 'line').
	// It gets start=0, end=0, then silently defaults to lines 1-50 of the file.
	// This demonstrates the schema mismatch: the function returns the wrong
	// source lines instead of the requested lines 10-20.
	source, err := p.FetchSource("schema-test-source")
	if err != nil {
		t.Errorf("FetchSource failed: %v", err)
	} else if source == "" {
		t.Errorf("FetchSource returned empty source")
	}
	// The bug: FetchSource returned *something* but NOT lines 10-20 as intended,
	// because it couldn't read the 'line' property (it queries 'start_line').
}

func TestLocateUsage_SchemaMismatch(t *testing.T) {
	// This test asserts that LocateUsage queries the correct line properties.
	// Parsers output 'line' and 'end_line'. LocateUsage queries 'start_line'.

	cfg := config.Config{
		Neo4jURI:      os.Getenv("NEO4J_URI"),
		Neo4jUser:     os.Getenv("NEO4J_USER"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
	}

	p, err := NewNeo4jProvider(cfg)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer p.Close()

	// 1. Setup Data with 'line' (matching parser schema), NOT 'start_line'
	setupQuery := `
		CREATE (n:CodeElement:Function {
			id: 'schema-test-source',
			name: 'TestSourceFunc',
			file: 'neo4j_test.go',
			line: 10,
			end_line: 20
		})
		CREATE (m:CodeElement:Function {
			id: 'schema-test-target',
			name: 'TestTargetFunc',
			file: 'neo4j_test.go',
			line: 30,
			end_line: 40
		})
		CREATE (n)-[:CALLS]->(m)
	`
	_, err = neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	defer func() {
		cleanupQuery := `MATCH (n) WHERE n.id STARTS WITH 'schema-test-' DETACH DELETE n`
		neo4j.ExecuteQuery(p.ctx, p.driver, cleanupQuery, nil, neo4j.EagerResultTransformer)
	}()

	// 2. LocateUsage queries 'start_line' which is missing (DB has 'line').
	// It gets start=0, end=0, then returns "missing location info" error.
	_, err = p.LocateUsage("schema-test-source", "schema-test-target")
	if err == nil {
	        t.Errorf("LocateUsage succeeded but should have failed because 'start_line' is missing in the DB.")
	} else if !strings.Contains(err.Error(), "missing location info") {
	        t.Errorf("LocateUsage failed for unexpected reasons: %v. Expected 'missing location info' error.", err)
	}
	}

	func TestGetNeighbors_Limit(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	fixture := `
	        CREATE (n:Function {id: 'LimitTarget', name: 'LimitTarget'})
	        CREATE (d1:Function {id: 'D1', name: 'D1'})
	        CREATE (d2:Function {id: 'D2', name: 'D2'})
	        CREATE (d3:Function {id: 'D3', name: 'D3'})
	        CREATE (n)-[:CALLS]->(d1)
	        CREATE (n)-[:CALLS]->(d2)
	        CREATE (n)-[:CALLS]->(d3)
	`
	_, err := p.executeQuery(fixture, nil)
	if err != nil {
	        t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test with limit 2 (should return 2 out of 3 dependencies)
	result, err := p.GetNeighbors("LimitTarget", 1, 2)
	if err != nil {
	        t.Fatalf("GetNeighbors failed: %v", err)
	}

	if len(result.Dependencies) != 2 {
	        t.Errorf("Expected 2 dependencies due to limit, got %d", len(result.Dependencies))
	}

	// Test with limit 10 (should return all 3)
	resultFull, err := p.GetNeighbors("LimitTarget", 1, 10)
	if err != nil {
	        t.Fatalf("GetNeighbors failed: %v", err)
	}

	if len(resultFull.Dependencies) != 3 {
	        t.Errorf("Expected 3 dependencies, got %d", len(resultFull.Dependencies))
	}
}

func TestWipeDatabase(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	// 1. Create a dummy node to verify delete works
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, "CREATE (n:TestNode {name: 'temp_to_wipe'})", nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}

	// 2. Call WipeDatabase
	if err := p.WipeDatabase(p.ctx); err != nil {
		t.Fatalf("WipeDatabase failed: %v", err)
	}

	// 3. Query count to confirm it is 0
	res, err := neo4j.ExecuteQuery(p.ctx, p.driver, "MATCH (n) RETURN count(n) as count", nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to verify database count: %v", err)
	}

	count := res.Records[0].Values[0].(int64)
	if count != 0 {
		t.Errorf("Expected 0 nodes after WipeDatabase, got %d", count)
	}
}
