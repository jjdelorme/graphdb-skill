//go:build integration

package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestCalculateRiskScores(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	cleanup(t, p)
	defer cleanup(t, p)

	// Setup fixture
	// f1: Volatile, high fan-in, high churn
	// f2: Non-volatile, low fan-in, low churn
	setupQuery := `
		CREATE (f1:Function {name: 'Test_HighRisk', id: 'f1', is_volatile: true})
		CREATE (file1:File {file: 'Test_churny.go', id: 'file1', change_frequency: 100})
		CREATE (f1)-[:DEFINED_IN]->(file1)
		
		CREATE (caller1:Function {name: 'Test_Caller1', id: 'c1'})
		CREATE (caller2:Function {name: 'Test_Caller2', id: 'c2'})
		CREATE (caller1)-[:CALLS]->(f1)
		CREATE (caller2)-[:CALLS]->(f1)

		CREATE (f2:Function {name: 'Test_LowRisk', id: 'f2', is_volatile: false})
		CREATE (file2:File {file: 'Test_stable.go', id: 'file2', change_frequency: 1})
		CREATE (f2)-[:DEFINED_IN]->(file2)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	err = p.CalculateRiskScores()
	if err != nil {
		t.Fatalf("CalculateRiskScores failed: %v", err)
	}

	// Verify
	verifyQuery := `
		MATCH (f:Function)
		WHERE f.name STARTS WITH 'Test_'
		RETURN f.name as name, f.risk_score as score
		ORDER BY f.risk_score DESC
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, verifyQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}

	if len(result.Records) < 2 {
		t.Fatalf("Expected at least 2 records, got %d", len(result.Records))
	}

	topName, _, _ := neo4j.GetRecordValue[string](result.Records[0], "name")
	if topName != "Test_HighRisk" {
		t.Errorf("Expected Test_HighRisk to have highest score, got %s", topName)
	}

	topScore, _, _ := neo4j.GetRecordValue[float64](result.Records[0], "score")
	if topScore <= 0 {
		t.Errorf("Expected positive risk score, got %f", topScore)
	}
}

func TestPropagateVolatility(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	cleanup(t, p)
	defer cleanup(t, p)

	// Setup fixture: UPWARD propagation
	// caller -> callee (volatile)
	setupQuery := `
		CREATE (caller:Function {name: 'Test_Caller', id: 'f1'})
		CREATE (callee:Function {name: 'Test_Callee', id: 'f2', is_volatile: true})
		CREATE (caller)-[:CALLS]->(callee)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	err = p.PropagateVolatility()
	if err != nil {
		t.Fatalf("PropagateVolatility failed: %v", err)
	}

	// Verify caller is now volatile
	verifyQuery := `
		MATCH (f:Function {name: 'Test_Caller'})
		RETURN f.is_volatile as is_volatile
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, verifyQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}

	if len(result.Records) == 0 {
		t.Fatal("Expected 1 record, got 0")
	}

	isVolatile, _, _ := neo4j.GetRecordValue[bool](result.Records[0], "is_volatile")
	if !isVolatile {
		t.Errorf("Expected caller to be volatile after upward propagation")
	}
}

func TestHasVolatilityData(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	cleanup(t, p)
	defer cleanup(t, p)

	// Initially, should be false
	hasData, err := p.HasVolatilityData()
	if err != nil {
		t.Fatalf("HasVolatilityData failed initially: %v", err)
	}
	if hasData {
		t.Errorf("Expected HasVolatilityData to be false initially")
	}

	// Create a node with is_volatile = false
	setupQuery := `
		CREATE (f:Function {name: 'Test_Func', id: 'f1', is_volatile: false})
	`
	_, err = neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Now it should be true, even though there are 0 volatile (true) functions
	hasData, err = p.HasVolatilityData()
	if err != nil {
		t.Fatalf("HasVolatilityData failed: %v", err)
	}
	if !hasData {
		t.Errorf("Expected HasVolatilityData to be true when a function has is_volatile=false")
	}

	// CountVolatileFunctions should be 0
	count, err := p.CountVolatileFunctions()
	if err != nil {
		t.Fatalf("CountVolatileFunctions failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected CountVolatileFunctions to be 0, got %d", count)
	}
}

