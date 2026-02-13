# Feature Implementation Plan: Parametric Traversal

## 📋 Todo Checklist
- [ ] **Step 1:** Define the Verification Harness (Black-box interface tests).
- [ ] **Step 2:** Update Interface Documentation & Helpers.
- [ ] **Step 3:** Implement `Traverse` in `internal/query/neo4j.go` (Cypher generation).
- [ ] **Step 4:** Integrate `traverse` command in `cmd/graphdb/main.go`.
- [ ] **Step 5:** Final System Verification (Run harness against implementation).

## 🔍 Analysis & Investigation

### Current State
*   **Interface:** `GraphProvider` in `internal/query/interface.go` already has the method signature: `Traverse(startNodeID string, relationship string, direction Direction, depth int) ([]*graph.Path, error)`.
*   **Implementation:** The method is currently stubbed in `internal/query/neo4j.go`.
*   **CLI:** `cmd/graphdb/main.go` handles query dispatching but lacks a handler for the `traverse` type.
*   **Graph Schema:** `internal/graph/schema.go` defines `Path`, `Node`, and `Edge`, which are sufficient for returning results.

### Requirements Mapping
*   **New Query Type:** `traverse`.
*   **Parameters:**
    *   `start_node`: Maps to `startNodeID` (and `-target` CLI flag).
    *   `edge_types`: Comma-separated list (e.g., "CALLS,USES"). Maps to `relationship` string in interface (will be parsed to Cypher pattern `[:CALLS|:USES]`).
    *   `direction`: Incoming/Outgoing/Both. Maps to `Direction` enum.
    *   `max_depth`: Maps to `depth` int.

### Architecture & Spanner Migration Strategy
To protect against future Spanner migration, we rely on the **Provider Pattern**:
1.  **Abstraction:** The `GraphProvider` interface defines *what* we want (a traversal), not *how* (Cypher/SQL).
2.  **Data Coupling:** The return type `[]*graph.Path` is a generic struct, decoupling the rest of the application from Neo4j's internal path representation.
3.  **Query Logic Isolation:** All Cypher logic is contained strictly within `internal/query/neo4j.go`. The CLI and other consumers just call `provider.Traverse`.
4.  **Verification:** The test harness will verify the behavior (e.g., "Finding downstream dependencies returns correct nodes") independent of the underlying database query language. When migrating to Spanner, we implement `SpannerProvider` satisfying the same test harness.

## 📝 Implementation Plan

### Prerequisites
*   Ensure `internal/graph/schema.go` defines `Path` (verified: it does).
*   Neo4j instance running (for integration testing).

### Step-by-Step Implementation

#### Phase 1: Verification Harness
1.  **Step 1.A (The Harness):** Create a new test file `internal/query/traversal_test.go`.
    *   *Action:* Define a test case that initializes a `GraphProvider` (mock or real) and asserts that `Traverse` returns expected paths for a known graph structure.
    *   *Goal:* Define the contract behaviorally.

#### Phase 2: Core Implementation
2.  **Step 2.A (Cypher Logic):** Implement `Traverse` in `internal/query/neo4j.go`.
    *   *Detail:*
        *   Parse `relationship` string (comma-separated) into Cypher relationship types (pipe-separated).
        *   Map `Direction` enum to Cypher arrow syntax (`<-[]-`, `-[]->`, `-[]-`).
        *   Execute Cypher query: `MATCH p = (start)-[r:TYPES*1..depth]-(end) WHERE ... RETURN p`.
        *   Map Neo4j `Path` result to `graph.Path`.
    *   *Protection:* Use parameterized queries for ID/Depth to prevent injection, but relationship types often need string interpolation in Cypher (with sanitization/validation).

3.  **Step 2.B (CLI Integration):** Update `cmd/graphdb/main.go`.
    *   *Action:* Add handling for `traverse` query type.
    *   *Detail:*
        *   Add flags: `-edge-types` (string), `-direction` (string).
        *   Parse direction string ("incoming", "outgoing", "both") to `query.Direction`.
        *   Call `provider.Traverse`.
        *   Output result as JSON.

#### Phase 3: Verification
4.  **Step 3.A (Manual Verification):**
    *   *Action:* Run `./bin/graphdb query -type traverse -target "SomeFunction" -edge-types "CALLS" -direction outgoing -depth 2`.
    *   *Success:* Returns JSON list of paths.

### Testing Strategy
*   **Unit Tests:** Verify `Direction` parsing logic.
*   **Integration Tests:** The `traversal_test.go` will act as an integration test if run against a live Neo4j instance (standard practice in this repo seems to rely on live DB for complex query tests or mocks for simple ones). We will ensure the code handles the `Path` mapping correctly.

## 🎯 Success Criteria
1.  `graphdb query -type traverse ...` works from the command line.
2.  Users can filter by specific edge types (e.g., only "CALLS").
3.  Users can specify direction (upstream/downstream analysis).
4.  The implementation does not leak Cypher details outside of `neo4j.go`.
