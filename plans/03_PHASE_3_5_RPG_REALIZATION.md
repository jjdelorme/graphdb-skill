# Feature Implementation Plan: RPG Realization (Prototype to Production)

## 📋 Todo Checklist
- [x] **Phase 1: Persistence & Wiring** (Stop the data leak)
- [ ] **Phase 2: Domain Discovery** (Real structure)
- [ ] **Phase 3: Semantic Clustering** (Real grouping)
- [ ] **Phase 4: LLM Integration** (Real intelligence)
- [ ] **Final Review:** E2E test with a real repository.

## 🔍 Analysis & Investigation
The current RPG implementation (`internal/rpg`) is a skeleton. It defines the types and flow but mocks the actual logic.
*   **Discovery:** Uses `SimpleDomainDiscoverer` (returns 1 root).
*   **Clustering:** Uses `SimpleClusterer` (returns 1 bucket).
*   **Enrichment:** Uses `MockSummarizer` (returns static strings).
*   **Persistence:** The `enrich-features` command calculates data but **discards it** (writes to stdout but not to the graph DB, and `Builder` doesn't even return the edges).

This campaign aims to replace these placeholders with functional, production-ready logic.

## 📝 Implementation Plan

### Prerequisites
*   `internal/embedding` package is working (for vectors).
*   `internal/storage` package is working (for JSONL emission).

### Step-by-Step Implementation

#### Phase 1: Persistence & Wiring (The Foundation)
**Goal:** Ensure that *if* we generate Features, they are actually saved to the database.
1.  **Step 1.A (The Harness):** Create a test that asserts `Builder.Build` returns both Nodes (Features) and Edges (IMPLEMENTS/PARENT_OF).
    *   *Action:* Update `internal/rpg/builder_test.go`.
    *   *Goal:* Fail if `Build` returns 0 edges for a non-empty input.
2.  **Step 1.B (The Implementation):** Fix `Builder.Build` to return Edges.
    *   *Action:* Modify `internal/rpg/builder.go`.
    *   *Detail:*
        *   When creating `Feature` nodes, track them.
        *   When clustering `Functions` into a `Feature`, create `IMPLEMENTS` edges.
        *   Return `[]graph.Node` and `[]graph.Edge`.
3.  **Step 1.C (The Verification):** Verify `enrich-features` command.
    *   *Action:* Run `go run cmd/graphdb/main.go enrich-features ...` and check output for edge JSONL.

#### Phase 2: Domain Discovery (The Structure)
**Goal:** Automatically identify high-level domains (bounded contexts) from the file structure.
1.  **Step 2.A (The Harness):** Create test for `DirectoryDomainDiscoverer`.
    *   *Action:* Create `internal/rpg/discovery_test.go`.
    *   *Goal:* Given a file list `["pkg/auth/login.go", "pkg/billing/invoice.go"]`, expect domains `auth` and `billing`.
2.  **Step 2.B (The Implementation):** Implement `DirectoryDomainDiscoverer`.
    *   *Action:* Create/Update `internal/rpg/discovery.go` (or similar).
    *   *Detail:*
        *   Implement logic to find common top-level directories or "seams".
        *   Initial heuristic: Top-level folders in `src/`, `pkg/`, or `internal/` are domains.
3.  **Step 2.C (The Switch):** Wire it up.
    *   *Action:* Update `cmd/graphdb/main.go` to use `DirectoryDomainDiscoverer` instead of `SimpleDomainDiscoverer`.

#### Phase 3: Semantic Clustering (The Grouping)
**Goal:** Group functions within a domain into cohesive "Features" based on semantic similarity (embeddings) or simple file locality.
1.  **Step 3.A (The Harness):** Create test for `SemanticClusterer`.
    *   *Action:* Create `internal/rpg/cluster_test.go`.
    *   *Goal:* Given 3 functions (2 related, 1 unrelated), expect 2 clusters.
2.  **Step 3.B (The Implementation):** Implement `HeuristicClusterer` (Iterative Step).
    *   *Action:* Create/Update `internal/rpg/cluster.go`.
    *   *Detail:*
        *   **Start Simple:** Cluster by **File** or **Package**. Functions in the same file belong to the same "Feature" initially.
        *   **Advanced (Later):** Use K-Means on embeddings if file-based is too granular. *Decision: Start with File-based clustering for reliability.*
3.  **Step 3.C (The Switch):** Wire it up.
    *   *Action:* Update `cmd/graphdb/main.go` to use the new Clusterer.

#### Phase 4: LLM Integration (The Intelligence)
**Goal:** Use the LLM to generate meaningful names and descriptions for the Features.
1.  **Step 4.A (The Harness):** Integration test with `internal/embedding` client.
    *   *Action:* Review `internal/rpg/enrich_test.go`. Ensure it mocks the `embedding.Embedder` or `vertex` client correctly.
2.  **Step 4.B (The Implementation):** Implement `VertexSummarizer`.
    *   *Action:* Update `internal/rpg/enrich.go`.
    *   *Detail:*
        *   Construct a prompt: "Here are the signatures of functions in this cluster: [...]. Name this feature and describe it."
        *   Call Vertex AI (reuse `internal/embedding` or `internal/llm` if it exists, otherwise instantiate client).
3.  **Step 4.C (The Switch):** Wire it up.
    *   *Action:* Update `cmd/graphdb/main.go` to use `VertexSummarizer`.

### Testing Strategy
*   **Unit Tests:** Verify each component (Discoverer, Clusterer, Summarizer) in isolation.
*   **Integration:** Run `enrich-features` on `internal/rpg` itself.
    *   Check if it creates a "RPG Core" domain.
    *   Check if it creates "Builder", "Enricher", "Schema" features.
    *   Check if `Feature` nodes have non-mock descriptions.

## 🎯 Success Criteria
*   **Persistence:** `graphdb enrich-features` outputs valid JSONL with `Feature` nodes and `IMPLEMENTS` edges.
*   **Structure:** The graph reflects the directory structure (Domains) and file structure (Features).
*   **Content:** Feature nodes have LLM-generated descriptions, not "Mock Feature".
