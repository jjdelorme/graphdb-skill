# Research Report: Missing Neo4j Import Capability in Go Binary

**Date:** 2023-10-27
**Subject:** Investigation of `graphdb` Go Binary Import Capabilities
**Target:** `cmd/graphdb` and `internal/`

## 1. Executive Summary
The `graphdb` Go binary **lacks the capability to import data into Neo4j**. While it can parse code and generate the required graph structure (Phase 1) and query an existing graph (Phase 2), the specific logic to *load* that generated data into the database remains in the legacy Node.js script (`import_to_neo4j.js`).

Current Workflow (Hybrid):
1.  **Extract:** `graphdb ingest` -> `graph.jsonl` (Go)
2.  **Load:** `node import_to_neo4j.js` -> Neo4j (Node.js)
3.  **Query:** `graphdb query` <- Neo4j (Go)

## 2. Evidence & Findings

### 2.1 Roadmap Status
Review of `plans/00_MASTER_ROADMAP.md` and `plans/01_PHASE_1_GO_INGESTOR.md` confirms this is by design (or deferred):
*   **Phase 1 Goal:** "JSONL output strictly matches existing schema... to decouple Extraction (Go) from Loading (Node.js/Neo4j)."
*   **Phase 2 Goal:** "Implement the 'Read' side... connecting to the backing store."
*   **Missing Item:** There is no completed task for "Port `import_to_neo4j.js` to Go".

### 2.2 Code Analysis
*   **`cmd/graphdb/main.go`**:
    *   Available commands: `ingest` (File -> JSONL), `query` (Neo4j -> Stdout), `enrich-features`.
    *   **Result:** No `import` or `sync` command exists.
*   **`internal/query/neo4j.go`**:
    *   Implements `GraphProvider` interface (Read-Only).
    *   Contains methods for `SearchFeatures`, `GetNeighbors`, `GetImpact`, `GetSeams`.
    *   **Result:** No write capability (no `CreateNode`, `MergeEdge`, etc.).
*   **`internal/storage/`**:
    *   Contains `jsonl.go` (Write to File).
    *   **Result:** No `neo4j_loader.go` or equivalent.

## 3. Required Implementation (Gap Analysis)
To retire the Node.js dependency completely, the following Go code must be written:

1.  **New Command:** Add `import` subcommand to `cmd/graphdb/main.go`.
2.  **New Package:** Create `internal/loader/` (or extend `internal/storage`).
3.  **Neo4j Writer Logic:**
    *   Read `graph.jsonl`.
    *   Batch `UNWIND` Cypher statements to insert Nodes and Edges efficiently.
    *   Handle constraints (uniqueness) and index creation.
    *   Mirror logic from `.gemini/skills/graphdb/scripts/import_to_neo4j.js`.

## 4. Recommendations for Architect
*   **Immediate Action:** Create a new plan/task to port `import_to_neo4j.js` to Go.
*   **Architecture:** Define a `Loader` interface in `internal/loader` to allow future support for Spanner Bulk Loading (Campaign 5) alongside Neo4j Import.
*   **Interim Fix:** Until the Go importer is ready, the `graphdb` skill wrapper (in `.gemini/skills/graphdb/scripts/orchestrate.js` or similar) must continue to call the Node.js import script after the Go binary finishes ingestion.
