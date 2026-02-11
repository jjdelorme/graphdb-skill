# Porting Requirements: Neo4j Import Loader (Campaign 4)

## Overview
This document outlines the functional requirements for porting `import_to_neo4j.js` to the Go binary (`graphdb`). The analysis is based on the current JavaScript implementation.

## 1. Batching Strategy (UNWIND)
The Go loader must implement a high-throughput batching mechanism to avoid round-trip latency.

*   **Logic:**
    1.  Read the input JSONL stream (Nodes or Edges).
    2.  Accumulate records into a buffer until `BATCH_SIZE` (default: 2000) is reached.
    3.  **Group by Type:** Inside the batch, records must be grouped by their `type` field (e.g., `structure`, `function`, `imports` for nodes; `defines`, `calls` for edges).
        *   *Reason:* Cypher labels/relationship types cannot be parameterized.
    4.  **Execute:** For each type group, construct and execute a Cypher query using `UNWIND`.

*   **Node Query Template:**
    ```cypher
    UNWIND $batch AS row
    MERGE (n:Entity {id: row.id})
    SET n += row
    // Dynamically append label:
    SET n:ACTUAL_TYPE_LABEL
    ```

*   **Edge Query Template:**
    ```cypher
    UNWIND $batch AS row
    MATCH (source:Entity {id: row.source})
    MATCH (target:Entity {id: row.target})
    // Dynamically append relationship type:
    MERGE (source)-[r:ACTUAL_REL_TYPE]->(target)
    ```

## 2. Clean / Incremental Logic
The current JS implementation is **"Clean by Default"**.

*   **Default Behavior (No Flag):**
    *   **Scope:** Full Database Wipe. It does *not* filter by project or directory.
    *   **Operation:**
        1.  Executes `CALL { MATCH (n) DETACH DELETE n } IN TRANSACTIONS OF 10000 ROWS`.
        2.  (Fallback) If the above fails, it iterates `MATCH (n) ... LIMIT 10000 DETACH DELETE n` until 0 nodes remain.
    *   **Schema:** Re-applies constraints after wiping.

*   **Incremental Mode (`--incremental`):**
    *   Skips the delete step.
    *   Skips constraint creation (assumes they exist).
    *   Proceeds directly to `MERGE` operations.

*   **Recommendation for Go Port:**
    *   Implement a `-clean` flag to mimic the default behavior (explicit is better than implicit).
    *   Default should likely be *incremental* (safe) or explicitly require a flag to wipe.
    *   **Risk:** The current wipe is global. If multiple projects share a DB, this is destructive. Future improvement: Scope deletion by `project_root` property on nodes.

## 3. GraphState Tracking
To enable "Smart Indexing" (Campaign 2), the loader tracks the Git commit of the ingested code.

*   **Source:** `git rev-parse HEAD` (executed in the target repo root).
*   **Storage:** A singleton `GraphState` node.
*   **Logic:**
    ```cypher
    MERGE (s:GraphState)
    SET s.last_indexed_commit = $commitHash,
        s.updated_at = timestamp()
    ```

## 4. Required Cypher Queries
The Go `neo4j` adapter must implement these specific queries:

| Purpose | Query / Logic | Notes |
| :--- | :--- | :--- |
| **Wipe DB** | `CALL { MATCH (n) DETACH DELETE n } IN TRANSACTIONS OF 10000 ROWS` | Fallback loop required if transaction fails. |
| **Constraint: ID** | `CREATE CONSTRAINT node_id IF NOT EXISTS FOR (n:Entity) REQUIRE n.id IS UNIQUE` | |
| **Index: Label** | `CREATE INDEX node_label IF NOT EXISTS FOR (n:Entity) ON (n.label)` | |
| **Index: File** | `CREATE INDEX node_file IF NOT EXISTS FOR (n:Entity) ON (n.file)` | |
| **Merge Nodes** | `UNWIND $batch AS row MERGE (n:Entity {id: row.id}) SET n += row, n:TYPE` | `TYPE` is dynamic. |
| **Merge Edges** | `UNWIND $batch AS row MATCH (s:Entity {id: row.source}) MATCH (t:Entity {id: row.target}) MERGE (s)-[r:TYPE]->(t)` | `TYPE` is dynamic. |
| **Get State** | `MATCH (s:GraphState) RETURN s.last_indexed_commit` | Used for staleness check. |
| **Set State** | `MERGE (s:GraphState) SET s.last_indexed_commit = $c, s.updated_at = timestamp()` | |

## 5. Implementation Notes
*   **Sanitization:** The JS code sanitizes dynamic labels: `key.replace(/[^a-zA-Z0-9_]/g, '')`. The Go port **MUST** do the same to prevent Cypher injection.
*   **Driver:** Use the official `github.com/neo4j/neo4j-go-driver/v5`.
*   **Config:** Respect standard env vars (`NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD`).
