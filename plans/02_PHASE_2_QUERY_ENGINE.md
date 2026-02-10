# Feature Implementation Plan: Phase 2 - The Graph Query Engine

**Campaign:** The Graph Query Engine (Campaign 2)
**Goal:** Implement the "Read" side of the platform in Go, mirroring the "Write" side (Ingestor). This enables the Go binary to answer queries directly by connecting to the backing store (initially Neo4j), paving the way for the Spanner migration.
**Context:** Currently, the `graphdb` skill relies on `query_graph.js` to execute Cypher queries. To make the Go binary a self-contained platform (and to support Spanner later), we must port this logic to Go behind an interface.

## 📋 Todo Checklist
- [ ] **Architecture:** Define the `GraphProvider` interface in `internal/query`.
- [ ] **Implementation:** Create `Neo4jProvider` implementing `GraphProvider`.
- [ ] **Feature:** Port `hybrid-context` (Vector + Structural Search).
- [ ] **Feature:** Port `test-context` (Dependency Traversal).
- [ ] **Feature:** Port `impact` (Reverse Dependency/Caller Analysis).
- [ ] **Feature:** Port `globals` (Global Variable Usage).
- [ ] **Feature:** Port `suggest-seams` (Clustering/Architecture Analysis).
- [ ] **CLI:** Add `query` subcommand to the Go binary.
- [ ] **Verification:** Integration tests against a running Neo4j instance (or mock).

## 🔍 Analysis & Investigation

### Architecture: The Provider Abstraction
We need a storage-agnostic way to query the graph. This allows us to swap Neo4j for Spanner in Campaign 4 without changing the business logic or the CLI interface.

```go
// internal/query/interface.go
type GraphProvider interface {
    // Core Graph Operations
    FindNode(label string, property string, value string) (*graph.Node, error)
    Traverse(startNodeID string, relationship string, direction Direction, depth int) ([]*graph.Path, error)
    
    // High-Level Features (Mirroring JS skill capabilities)
    SearchFeatures(embedding []float32, limit int) ([]*FeatureResult, error) // hybrid-context
    GetNeighbors(nodeID string) (*NeighborResult, error)                     // part of test-context
    GetImpact(nodeID string) (*ImpactResult, error)                          // impact
    GetGlobals(nodeID string) (*GlobalUsageResult, error)                    // globals
    SuggestSeams() (*SeamResult, error)                                      // suggest-seams
    
    // Lifecycle
    Close() error
}
```

### Porting Logic: JS to Go
We are porting `query_graph.js`. The Go implementation must support full feature parity with the existing critical query types:

1.  **hybrid-context (Vector + Structural):**
    *   *JS Logic:* `CALL db.index.vector.queryNodes('function_embeddings', 5, target.embedding)` combined with structural filtering.
    *   *Go Logic:* `Neo4jProvider` will construct the Cypher query to combine vector similarity with graph traversal.
2.  **test-context (Dependencies):**
    *   *JS Logic:* `MATCH path = (f)-[:CALLS*0..depth]->(callee)`
    *   *Go Logic:* Traverse downstream dependencies to find required context for tests.
3.  **impact (Reverse Dependencies):**
    *   *JS Logic:* `MATCH path = (caller)-[:CALLS*0..depth]->(f)`
    *   *Go Logic:* Traverse upstream (reverse) to find code impacted by a change.
4.  **globals (Global Usage):**
    *   *JS Logic:* `MATCH (f)-[:USES_GLOBAL]->(g)`
    *   *Go Logic:* Identify global variables used by a specific function or set of functions.
5.  **suggest-seams (Clustering):**
    *   *JS Logic:* Community detection (e.g., Louvain or WCC) to find natural architectural boundaries.
    *   *Go Logic:* Execute graph algorithms via Cypher projection (`gds.louvain` or similar) or emulate basic clustering if GDS is unavailable.

### Dependencies
*   `github.com/neo4j/neo4j-go-driver/v5`: Official Go driver for Neo4j.

## 📝 Implementation Plan

### Prerequisites
*   Phase 1 (Go Ingestor) structure in place.
*   Running Neo4j instance (local) for integration testing.

### Project Structure (Updates)
```
/
├── cmd/
│   └── graphdb/
│       └── main.go           # Add 'query' subcommand
├── internal/
│   ├── query/                # NEW PACKAGE
│   │   ├── interface.go      # GraphProvider interface
│   │   └── neo4j.go          # Neo4j implementation
│   ├── graph/                # Existing data models
│   └── config/               # Configuration loading (env vars)
```

### Step-by-Step Implementation

#### Phase 2.1: The Interface & Connection
1.  **Step 2.1.A (Red):** Define Interface and Configuration.
    *   *Action:* Create `internal/query/interface.go` with methods for all 5 query types.
    *   *Action:* Create `internal/config/loader.go` to load `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD`.
    *   *Test:* Create `internal/config/loader_test.go` to verify env var loading.
2.  **Step 2.1.B (Green):** Implement Neo4j Connection.
    *   *Action:* Create `internal/query/neo4j.go`. Implement `NewNeo4jProvider(config)`.
    *   *Action:* Implement `Close()`.
    *   *Verify:* Write a manual integration test (skipped if env vars missing) that connects to localhost and runs `RETURN 1`.

#### Phase 2.2: Context Queries (Hybrid & Test)
1.  **Step 2.2.A (Red):** Define Context Search requirement.
    *   *Test:* Create `internal/query/neo4j_test.go` (Integration).
    *   *Logic:* Insert a dummy node with embedding. Call `SearchFeatures` (hybrid). Assert node is returned.
2.  **Step 2.2.B (Green):** Implement Search Logic.
    *   *Action:* Implement `SearchFeatures` and `GetNeighbors` in `neo4j.go`.
    *   *Cypher:* Construct `CALL db.index.vector.queryNodes` and `MATCH (f)-[:CALLS*]...`.
    *   *Verify:* Run the integration test.

#### Phase 2.3: Impact & Global Queries
1.  **Step 2.3.A (Red):** Define Impact/Globals requirement.
    *   *Test:* Add to `internal/query/neo4j_test.go`. Insert Caller->Callee and Func->Global.
    *   *Logic:* Call `GetImpact` on Callee (expect Caller). Call `GetGlobals` on Func (expect Global).
2.  **Step 2.3.B (Green):** Implement Logic.
    *   *Action:* Implement `GetImpact` (reverse traversal) and `GetGlobals` in `neo4j.go`.
    *   *Verify:* Run integration test.

#### Phase 2.4: Architecture Analysis (Seams)
1.  **Step 2.4.A (Red):** Define Seams requirement.
    *   *Test:* Add to `internal/query/neo4j_test.go`. Create two disconnected clusters of nodes.
    *   *Logic:* Call `SuggestSeams`. Expect result to identify two groups.
2.  **Step 2.4.B (Green):** Implement Seams Logic.
    *   *Action:* Implement `SuggestSeams` in `neo4j.go` using `gds` or fallback logic.
    *   *Verify:* Run integration test.

#### Phase 2.5: CLI Wiring
1.  **Step 2.5.A (Red):** CLI Command test.
    *   *Test:* `test/e2e/cli_test.go`. Run `graphdb query --type=impact --target=[...]`.
2.  **Step 2.5.B (Green):** Implement Subcommands.
    *   *Action:* Update `cmd/graphdb/main.go` to use `spf13/cobra` or standard `flag.FlagSet` for subcommands.
    *   *Logic:* Wire `query` command to `GraphProvider` and handle flags for all 5 query types.

### Testing Strategy
*   **Unit Tests:** Use a mock for `neo4j.Driver` (if possible) or focus on the Query Builder logic if extracted.
*   **Integration Tests:** This is the primary verification method for this phase.
    *   Use `testcontainers-go` to spin up a Neo4j Docker container during tests if CI allows.
    *   Fallback: Skip tests if `NEO4J_URI` is not set, allowing local dev testing against the standard dev database.

## 🎯 Success Criteria
1.  **Full Parity:** The Go binary returns the same results as `query_graph.js` for all 5 query types (`hybrid-context`, `test-context`, `impact`, `globals`, `suggest-seams`).
2.  **Interface Abstraction:** The `GraphProvider` interface does not leak Neo4j specifics.
3.  **Stability:** Handles connection errors and invalid queries gracefully.
