# Feature Implementation Plan: CLI RPG Updates & Parity

## 📋 Todo Checklist
- [ ] **Command: `enrich-features`**: Implement new CLI command in `cmd/graphdb/main.go`.
- [ ] **Query: `search-features`**: Rename/expose `features` query type.
- [ ] **Query: `search-similar`**: Expose `SearchSimilarFunctions`.
- [ ] **Query: `hybrid-context`**: Implement combined semantic + structural search.
- [ ] **Query: `test-context`**: Add alias for `neighbors`.
- [ ] **Verification**: Ensure all new commands and query types work as expected.

## 🔍 Analysis & Investigation
The current `cmd/graphdb/main.go` supports `ingest` and `query` commands. The `query` command has basic support for `features`, `neighbors`, `impact`, `globals`, `seams`. We need to expand this to support the new RPG capabilities and match the JS skill's behavior.

### `enrich-features` Command
*   **Goal:** Run the RPG construction pipeline (Top-Down Seeding + Clustering + LLM labeling).
*   **Implementation:**
    *   Add `enrich-features` case to `main` switch.
    *   Parse flags: `-dir`, `-project`, `-location`, `-mock-embedding`, `-token`.
    *   Instantiate `rpg.Builder` with basic implementations of `DomainDiscoverer` and `Clusterer`.
    *   Instantiate `rpg.Enricher` with basic `Summarizer`.
    *   Load graph data (e.g., from `graph.jsonl`) to get function nodes.
    *   Run `builder.Build` and `enricher.Enrich`.
    *   Output the resulting feature hierarchy (e.g., to JSON).

### Query Types
*   **`search-features`**: Maps to `provider.SearchFeatures` (already implemented in `internal/query/neo4j.go`).
*   **`search-similar`**: Maps to `provider.SearchSimilarFunctions` (already implemented in `internal/query/neo4j.go`).
*   **`hybrid-context`**: Matches JS behavior. Calls `SearchSimilarFunctions` (semantic) and `GetNeighbors` (structural) and combines results.
*   **`test-context`**: Alias for `neighbors` (structural only).

## 📝 Implementation Plan

### Prerequisites
*   `internal/rpg` package exists (verified).
*   `internal/query` package exists (verified).

### Step-by-Step Implementation

#### Phase 1: `enrich-features` Command
1.  **Step 1.A (The Harness):** Create a test case (or manual verification script) to run `enrich-features`.
    *   *Action:* Create `test/e2e/cli_enrich_test.go` (if not exists) or update `test/e2e/cli_test.go`.
    *   *Goal:* Verify the command runs without error.
2.  **Step 1.B (The Implementation):** Implement `handleEnrichFeatures` in `cmd/graphdb/main.go`.
    *   *Action:* Add `SimpleDomainDiscoverer` and `SimpleClusterer` structs (placeholders).
    *   *Action:* Implement logic to read `graph.jsonl`, build features, and print output.
    *   *Detail:* Use `rpg.Builder` and `rpg.Enricher`.

#### Phase 2: Query Updates
1.  **Step 2.A (The Harness):** Update `test/e2e/cli_test.go` to test new query types.
    *   *Action:* Add tests for `search-features`, `search-similar`, `hybrid-context`.
2.  **Step 2.B (The Implementation):** Update `handleQuery` in `cmd/graphdb/main.go`.
    *   *Action:* Rename `features` to `search-features`.
    *   *Action:* Add `search-similar` case.
    *   *Action:* Add `hybrid-context` case (calls both methods, combines results).
    *   *Action:* Add `test-context` alias.

### Testing Strategy
*   Run `go build ./cmd/graphdb`.
*   Run `./graphdb enrich-features ...`.
*   Run `./graphdb query -type search-features ...`.
*   Run `./graphdb query -type search-similar ...`.
*   Run `./graphdb query -type hybrid-context ...`.

## 🎯 Success Criteria
*   `enrich-features` command runs and produces output.
*   New query types (`search-features`, `search-similar`, `hybrid-context`) return correct results (or errors if DB not ready, but logic flow is correct).
*   `plans/04_PHASE_4_GEMINI_INTEGRATION.md` is updated.
