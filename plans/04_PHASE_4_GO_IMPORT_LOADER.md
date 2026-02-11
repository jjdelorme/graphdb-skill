# Feature Implementation Plan: Go Import Loader

## 📋 Todo Checklist
- [x] Phase 1: The Loader Package (`internal/loader`)
- [x] Phase 2: The CLI Command (`cmd/graphdb`)
- [x] Phase 3: Parity & Optimization (Wipe, Clean, Constraints)
- [ ] Final Review and Testing

## 🔍 Analysis & Investigation
The goal is to port the functionality of `import_to_neo4j.js` to the Go binary `graphdb`. This involves reading JSONL files (nodes and edges), batching them, and inserting them into Neo4j using the official Go driver.

### Key Requirements
1.  **High-Throughput Batching:** Use `UNWIND` for bulk inserts.
2.  **Clean/Incremental Modes:** Support wiping the DB (`-clean`) vs. incremental updates.
3.  **Schema Management:** Apply constraints and indexes.
4.  **Graph State:** Track commit hash.
5.  **Sanitization:** Prevent Cypher injection.

### Architecture
*   **`internal/loader`:** New package containing the core logic (`Neo4jLoader`).
*   **`cmd/graphdb`:** New `import` command handling flag parsing and file reading.
*   **`internal/config`:** Reuse existing config loading.

## 📝 Implementation Plan

### Prerequisites
*   Neo4j instance available (local or remote).
*   `graphdb` binary compiling.

### Step-by-Step Implementation

#### Phase 1: The Loader Package
1.  **Step 1.A (The Harness):** Define the verification requirement.
    *   *Action:* Create `internal/loader/neo4j_loader_test.go`.
    *   *Goal:* Define interface and mock tests for `BatchLoad`.
2.  **Step 1.B (The Implementation):** Create `internal/loader/neo4j_loader.go`.
    *   *Action:* Implement `Neo4jLoader` struct with `BatchLoad` method.
    *   *Detail:*
        *   Accept `[]map[string]any`.
        *   Group by `type` (for dynamic labeling).
        *   Construct `UNWIND` queries safely.
        *   Execute transaction.
3.  **Step 1.C (The Verification):** Verify the harness.
    *   *Action:* Run `go test ./internal/loader/...`.
    *   *Success:* Tests pass.

#### Phase 2: The CLI Command
1.  **Step 2.A (The Harness):** Define CLI behavior.
    *   *Action:* Update `test/e2e/cli_test.go` (or create new) to test `graphdb import --help`.
2.  **Step 2.B (The Implementation):** Implement `handleImport` in `cmd/graphdb/main.go`.
    *   *Action:* Add `import` subcommand.
    *   *Detail:*
        *   Flags: `-nodes`, `-edges`, `-clean`, `-batch-size`.
        *   Logic: Read JSONL line-by-line -> Buffer -> Call `loader.BatchLoad`.
3.  **Step 2.C (The Verification):** Verify CLI.
    *   *Action:* Run `go run cmd/graphdb/main.go import --help`.
    *   *Success:* Command is recognized and prints usage.

#### Phase 3: Parity & Optimization
1.  **Step 3.A (The Harness):** Test Wipe and Constraints.
    *   *Action:* Add tests to `internal/loader/neo4j_loader_test.go` for `Wipe` and `ApplyConstraints`.
2.  **Step 3.B (The Implementation):** Implement advanced features.
    *   *Action:* Add methods to `Neo4jLoader`.
    *   *Detail:*
        *   `Wipe()`: `CALL { MATCH (n) DETACH DELETE n } ...`.
        *   `ApplyConstraints()`: Create uniqueness constraints on `id` and indexes on `label`, `file`.
        *   `UpdateGraphState(commit string)`: Merge `GraphState` node.
3.  **Step 3.C (The Verification):** Verify full flow.
    *   *Action:* Run full integration test against local Neo4j (if available) or mock.

### Testing Strategy
*   **Unit Tests:** Focus on `internal/loader` query generation (ensure correct Cypher syntax).
*   **Integration Tests:**
    *   Requires a running Neo4j container.
    *   Script: `scripts/test_loader.sh` (to be created) that starts Neo4j, runs `graphdb import`, and queries Neo4j to verify data.
*   **Manual Verification:**
    *   Generate `graph.jsonl` using `graphdb ingest`.
    *   Load it using `graphdb import`.
    *   Check Neo4j Browser for nodes/edges.

## 🎯 Success Criteria
*   `graphdb import` successfully loads `graph.jsonl` into Neo4j.
*   `-clean` flag wipes the database before loading.
*   Constraints and Indexes are applied.
*   Performance is comparable to or better than the JS implementation.
