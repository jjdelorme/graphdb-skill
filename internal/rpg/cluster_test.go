package rpg

import (
	"graphdb/internal/graph"
	"testing"
)

func TestFileClusterer_Cluster(t *testing.T) {
	clusterer := &FileClusterer{}

	nodes := []graph.Node{
		{ID: "f1", Properties: map[string]interface{}{"file": "internal/auth/login.go"}},
		{ID: "f2", Properties: map[string]interface{}{"file": "internal/auth/login.go"}},
		{ID: "f3", Properties: map[string]interface{}{"file": "internal/auth/session.go"}},
		{ID: "f4", Properties: map[string]interface{}{"file": "no_path"}},
	}

	clusters, err := clusterer.Cluster(nodes, "auth")
	if err != nil {
		t.Fatalf("Cluster failed: %v", err)
	}

	if len(clusters["login"]) != 2 {
		t.Errorf("Expected 2 nodes in 'login' cluster, got %d", len(clusters["login"]))
	}
	if len(clusters["session"]) != 1 {
		t.Errorf("Expected 1 node in 'session' cluster, got %d", len(clusters["session"]))
	}
	// Note: 'no_path' is a valid filename, so it will cluster as 'no_path'
	if len(clusters["no_path"]) != 1 {
		t.Errorf("Expected 1 node in 'no_path' cluster, got %d", len(clusters["no_path"]))
	}
}
