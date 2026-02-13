package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestTraverse(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup fixture data
	// A -> B (CALLS)
	// B -> C (CALLS)
	// C -> D (USES)
	setupQuery := `
		CREATE (a:Function {name: 'TestA', id: 'TestA'})
		CREATE (b:Function {name: 'TestB', id: 'TestB'})
		CREATE (c:Function {name: 'TestC', id: 'TestC'})
		CREATE (d:Global {name: 'TestD', id: 'TestD'})
		CREATE (a)-[:CALLS]->(b)
		CREATE (b)-[:CALLS]->(c)
		CREATE (c)-[:USES]->(d)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	t.Run("Outgoing CALLS from A depth 2", func(t *testing.T) {
		paths, err := p.Traverse("TestA", "CALLS", Outgoing, 2)
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		// Expected paths: (A)->(B), (A)->(B)->(C)
		if len(paths) != 2 {
			t.Errorf("Expected 2 paths, got %d", len(paths))
		}

		foundB := false
		foundC := false
		for _, path := range paths {
			if len(path.Nodes) == 2 && path.Nodes[1].ID == "TestB" {
				foundB = true
			}
			if len(path.Nodes) == 3 && path.Nodes[2].ID == "TestC" {
				foundC = true
			}
		}

		if !foundB {
			t.Error("Did not find path to TestB")
		}
		if !foundC {
			t.Error("Did not find path to TestC")
		}
	})

	t.Run("Incoming CALLS to C depth 2", func(t *testing.T) {
		paths, err := p.Traverse("TestC", "CALLS", Incoming, 2)
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		// Expected paths: (B)->(C), (A)->(B)->(C)
		// Note: Cypher usually returns paths starting from the start node, even if direction is incoming
		// depending on how the query is written. We should decide on the convention.
		// For 'Incoming', the start node is the TARGET.
		if len(paths) != 2 {
			t.Errorf("Expected 2 paths, got %d", len(paths))
		}
	})

	t.Run("Both CALLS and USES from B depth 2", func(t *testing.T) {
		paths, err := p.Traverse("TestB", "CALLS,USES", Outgoing, 2)
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		// Expected paths: (B)->(C), (B)->(C)->(D)
		if len(paths) != 2 {
			t.Errorf("Expected 2 paths, got %d", len(paths))
		}
		
		foundD := false
		for _, path := range paths {
			if len(path.Nodes) == 3 && path.Nodes[2].ID == "TestD" {
				foundD = true
			}
		}
		if !foundD {
			t.Error("Did not find path to TestD via USES")
		}
	})
}
