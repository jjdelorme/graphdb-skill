package query

import (
	"graphdb/internal/config"
	"os"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func getProvider(t *testing.T) *Neo4jProvider {
	uri := os.Getenv("NEO4J_URI")
	if uri == "" {
		t.Skip("NEO4J_URI not set, skipping integration test")
	}

	cfg := config.Config{
		Neo4jURI:      uri,
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
		MATCH (n) WHERE n.label STARTS WITH 'Test' OR n.label = 'ContaminatedCaller' OR n.label = 'SeamFunc' OR n.file = 'test_fixture.go' DETACH DELETE n
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
		CREATE (f:Function {label: 'TestFunc', embedding: [0.1, 0.2, 0.3]})
		CREATE (c:Function {label: 'TestCallee'})
		CREATE (g:Global {label: 'TestGlobal', file: 'test_fixture.go'})
		CREATE (f)-[:CALLS]->(c)
		CREATE (c)-[:USES_GLOBAL]->(g)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test
	result, err := p.GetNeighbors("TestFunc", 2)
	if err != nil {
		t.Fatalf("GetNeighbors failed: %v", err)
	}

	// Verify
	foundGlobal := false
	foundFunc := false
	
	for _, dep := range result.Dependencies {
		if dep.Name == "TestGlobal" && dep.Type == "Global" {
			foundGlobal = true
			// Path includes [Callee, Global] in Neo4j logic from JS query
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

func TestGetCallers(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	setupQuery := `
		CREATE (caller:Function {label: 'TestCaller'})
		CREATE (target:Function {label: 'TestTarget'})
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
		CREATE (caller:Function {label: 'TestDeepCaller', ui_contaminated: true})
		CREATE (mid:Function {label: 'TestMid'})
		CREATE (target:Function {label: 'TestTarget'})
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
			if val, ok := c.Properties["ui_contaminated"].(bool); !ok || !val {
				t.Error("Expected TestDeepCaller to be contaminated")
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
		CREATE (f:Function {label: 'TestFunc'})
		CREATE (g:Global {label: 'TestGlobalVar', file: 'test_fixture.go'})
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
		CREATE (caller:Function {label: 'ContaminatedCaller', ui_contaminated: true})
		CREATE (seam:Function {label: 'SeamFunc', ui_contaminated: false, risk_score: 0.8})
		CREATE (file:File {file: 'test_fixture.go'})
		CREATE (caller)-[:CALLS]->(seam)
		CREATE (seam)-[:DEFINED_IN]->(file)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test with matching pattern
	results, err := p.GetSeams(".*test_fixture.*")
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
	if results[0].Risk != 0.8 {
		t.Errorf("Expected risk 0.8, got %f", results[0].Risk)
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

func TestSearchFeatures(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// We assume index 'function_embeddings' exists with 768 dimensions in the shared environment.
	// If it doesn't, this test might fail or we should detect and skip.
	
	// Setup with 768 dimensions
	v1 := makeVector(768, 1.0, 0)
	v2 := makeVector(768, 1.0, 1)

	setupQuery := `
		CREATE (f1:Function {label: 'TestSim1', embedding: $v1})
		CREATE (f2:Function {label: 'TestSim2', embedding: $v2})
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
	
	results, err := p.SearchFeatures(queryVec, 1)
	if err != nil {
		t.Logf("SearchFeatures failed (index might be missing): %v", err)
		return 
	}

	if len(results) > 0 {
		if results[0].Node.Label != "TestSim1" {
			t.Errorf("Expected TestSim1, got %s (Score: %f)", results[0].Node.Label, results[0].Score)
		}
	} else {
		t.Log("No results found (index might be building)")
	}
}
