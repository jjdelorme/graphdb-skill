# Feature Implementation Plan: The RPG Core (Intent Layer)

## 📋 Todo Checklist
- [x] **Schema Definition:** Define `Feature` structures and Graph relationships in Go.
- [x] **Hybrid Construction:** Implement the "Top-Down Seeding" + "Constrained Clustering" pipeline.
- [x] **Enrichment Loop:** Implement `enrich_features` command (LLM integration).
- [x] **Search Capability:** Implement `search_features` query in `GraphProvider`.
- [x] **Final Review:** Verify end-to-end RPG generation on a sample repository.

## 🔍 Analysis & Investigation
The Repository Planning Graph (RPG) introduces a "Functional View" (Intent) alongside the existing "Dependency View" (Implementation). This requires a fundamental schema change (`Feature` nodes) and a new construction pipeline.

### Findings
*   **Current Schema:** Generic `Node` struct in `internal/graph/schema.go`. We need a typed `Feature` representation for internal logic.
*   **Construction Strategy:** Pure bottom-up clustering is noisy. We will adopt the **Hybrid Construction** strategy (Top-Down Seeding -> Constrained Clustering) recommended in `RPG_ANALYSIS.md`.
*   **Gap:** The Go binary currently only supports `extract` (Ingestion) and `query` (Read). It lacks the "Synthesize" capability (LLM loop) required for RPG.

### Architecture
1.  **New Package:** `internal/rpg/` to house the core logic.
2.  **Schema:**
    *   `Feature` Node: `{ id, name, description, embedding, scope_path }`
    *   Edges: `PARENT_OF` (Feature->Feature), `IMPLEMENTS` (Function->Feature).
3.  **Pipeline:**
    *   `TopDownSeeding`: LLM generates top-level domains from file tree.
    *   `ConstrainedClustering`: K-Means groups code within domains.
    *   `SemanticRouting`: Assigns code to best-fit Feature.

## 📝 Implementation Plan

### Prerequisites
*   Existing `GraphProvider` interface (Campaign 2).
*   Vertex AI client (already in `internal/embedding`).

### Step-by-Step Implementation

#### Phase 1: Schema & Core Types
1.  **Step 1.A (The Harness):** Define test for Feature serialization.
    *   *Action:* Create `internal/rpg/schema_test.go`.
    *   *Goal:* Assert that a `Feature` struct can be serialized to the expected JSONL format for the graph DB.
2.  **Step 1.B (The Implementation):** Create Feature structures.
    *   *Action:* Create `internal/rpg/schema.go`.
    *   *Detail:* Define `Feature` struct with fields: `Name`, `Description`, `Embedding` ([]float32), `ScopePath`. implement conversion to `graph.Node`.
3.  **Step 1.C (The Verification):** Run the serialization test.
    *   *Action:* Run `go test ./internal/rpg/...`.

#### Phase 2: Hybrid Construction Pipeline
1.  **Step 2.A (The Harness):** Create a mock for the Clustering Service.
    *   *Action:* Create `internal/rpg/clustering_test.go`.
    *   *Goal:* Define inputs (list of Functions) and expected outputs (Tree of Features).
2.  **Step 2.B (The Implementation):** Implement Top-Down Seeding.
    *   *Action:* Create `internal/rpg/builder.go`.
    *   *Detail:* Implement `DiscoverDomains(fileTree)` which calls LLM to get root domains.
3.  **Step 2.C (The Implementation):** Implement Constrained Clustering.
    *   *Action:* Update `internal/rpg/builder.go`.
    *   *Detail:* Implement `ClusterFunctions(domain, functions)` using K-Means (or similar simple algorithm initially) to group functions into Features.
4.  **Step 2.D (The Verification):** Verify the Builder.
    *   *Action:* Run `go test ./internal/rpg/...` ensuring a mock list of functions produces a logical tree.

#### Phase 3: The Enrichment Loop (LLM Integration)
1.  **Step 3.A (The Harness):** Mock the Vertex AI client for text generation.
    *   *Action:* Update `internal/rpg/enrich_test.go`.
    *   *Goal:* Ensure `GenerateDescription` handles LLM responses correctly.
2.  **Step 3.B (The Implementation):** Implement Feature Enrichment.
    *   *Action:* Create `internal/rpg/enrich.go`.
    *   *Detail:* Implement `EnrichFeature(feature, children)` which generates a summary name and description.
3.  **Step 3.C (The Verification):** Run the enrichment test.

#### Phase 4: Integration & Query
1.  **Step 4.A (The Harness):** Define the `SearchFeatures` contract.
    *   *Action:* Update `internal/query/interface_test.go`.
    *   *Goal:* Assert `SearchFeatures("auth")` returns relevant Feature nodes.
2.  **Step 4.B (The Implementation):** Implement Query Logic.
    *   *Action:* Update `internal/query/neo4j.go` (and other providers).
    *   *Detail:* Implement Cypher query to vector search against `Feature` nodes and return the subgraph.
3.  **Step 4.C (The Verification):** Integration Test.
    *   *Action:* Run `go test ./internal/query/...`.

### Testing Strategy
*   **Unit Tests:** specific tests for `rpg` package (Schema, Builder, Enricher).
*   **Mocking:** Heavy use of mocks for LLM calls (Vertex AI) to ensure deterministic tests and avoid costs.
*   **Integration:** Use the `test/fixtures` to generate a small RPG and verify structure.

## 🎯 Success Criteria
*   Running `graphdb enrich-features` on a repo generates a `Feature` node hierarchy in the database.
*   `Feature` nodes are linked to `Function` nodes via `IMPLEMENTS`.
*   `graphdb query search-features "login"` returns the "Authentication" feature node.
