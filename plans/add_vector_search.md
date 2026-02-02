# Plan: Hybrid Graph with Vector Search (GraphRAG)

**Goal:** Enhance the existing Code Property Graph (CPG) with semantic vector embeddings to generate **Refactoring-Ready Context**. This allows the agent to identify implicit dependencies (cross-language calls, logical clones) that static analysis misses, facilitating safe refactoring of legacy code.

**Target Model:** `gemini-embedding-001` via Vertex AI.
**Library:** `google.genai` (Node.js SDK) or Google Cloud Vertex AI REST API.
**Authentication:** **Vertex AI ONLY** (Google Application Default Credentials - ADC).
**Database:** Neo4j Community Edition (v5.15+ required for Vector Indexes).

---

## 1. Rationale: Why Hybrid?

Many target projects are mature "Legacy Systems" with mixed languages or complex architectures. Static analysis (`query_graph.js` today) only sees *explicit* structural links. It fails to catch:
1.  **Logical Clones:** Copy-pasted business logic (common in legacy code) that must be refactored together.
2.  **Implicit Coupling:** Dependencies via database state, shared strings, or loosely coupled events.
3.  **Cross-Language Semantics:** Linking logic in one language (e.g., C++) to tests or related modules in another (e.g., C#) based on meaning, not just symbols.

By merging **Structural** (Callers/Callees) and **Semantic** (Vector Similarity) data, we provide the LLM with a complete "Context Window" for safe code modification.

---

## 2. Architecture Update

We will move from a pure **Structural Graph** to a **Hybrid Graph**.

### New Components
1.  **`VectorService`**: A Node.js module interacting with Google Vertex AI.
    *   **CRITICAL:** Must implement **Exponential Backoff** and **Retry Logic** to handle HTTP 429 (Quota Exceeded) errors robustly.
2.  **`enrich_vectors.js`**: A standalone script that iterates existing `Function` nodes, reads their source code from disk, generates embeddings, and updates the graph.
3.  **`query_graph.js` (Enhanced)**: updated to support a `hybrid-context` command that merges structural neighbors with semantic neighbors.

### Schema Changes (Neo4j)
*   **Node Property**: `Function.embedding` (Vector<Float>, 768 dimensions for `gemini-embedding-001`).
*   **Index**: `function_embeddings` (Vector Index).

---

## 3. Test-First Strategy (TDD)

We will harden the existing prototype code by adding missing test coverage before refactoring.

### A. Update `VectorService.test.js`
*   **Existing Tests**: Configuration, Embedding Generation, Batching.
*   **New Test (Critical)**: **Rate Limit Handling**.
    *   Mock a 429 error from the API.
    *   Assert the service waits (backoff) and retries.
    *   Assert it eventually succeeds or fails after Max Retries.

### B. Update `EnrichmentLogic.test.js` (or create if missing)
*   **Test 1: Source Extraction**: Verify it respects the `start_line` / `end_line` from the graph.
*   **Test 2: Safety Filter**: Verify the logic (or Cypher) explicitly excludes `node_modules` or other ignored directories.

---

## 4. Implementation Plan

### Phase 0: Environment Setup (Skill Stability)

Before any refactoring, we must ensure the skill's local environment is stable.
1.  **Install Dependencies**:
    *   Command: `npm install` inside `.gemini/skills/graphdb/`.
    *   Requirement: Ensure `tree-sitter`, `neo4j-driver`, `dotenv`, and `@google/genai` are correctly linked.
2.  **Verify Baseline**:
    *   Run `npm test --prefix .gemini/skills/graphdb` and ensure all existing extraction tests pass.

### Phase 1: Preparation & Refactoring (The "Generalized" Integration)

1.  **Refactor for `Neo4jService`**:
    *   The scripts `enrich_vectors.js` and `find_implicit_links.js` currently manually instantiate `neo4j.driver`.
    *   **Action**: Update them to import and use the shared `.gemini/skills/graphdb/scripts/Neo4jService.js` to respect global config and connection pooling.
2.  **Verify Environment**:
    *   Ensure `GOOGLE_CLOUD_PROJECT` and `GOOGLE_CLOUD_LOCATION` are set for Vertex AI.

### Phase 2: Vector Service Hardening

**File:** `.gemini/skills/graphdb/scripts/services/VectorService.js`

*   **Current State**: Basic implementation exists but lacks retry logic.
*   **Upgrade**: Implement **Exponential Backoff**.
    *   Add `sleep` utility.
    *   Wrap `embedContent` call in a loop.
    *   Handle `429` specifically.
    *   Handle `null` returns from the API gracefully.

### Phase 3: Enrichment Script Upgrades

**File:** `.gemini/skills/graphdb/scripts/enrich_vectors.js`

*   **Current State**: Basic loop exists.
*   **Upgrade**:
    1.  **Inject Filters**: Add `AND NOT f.file CONTAINS 'node_modules'` to the `MATCH` query.
    2.  **Integrate Neo4jService**: (As per Phase 1).
    3.  **Error Handling**: Ensure file read errors don't crash the entire batch.

### Phase 4: Integration into `query_graph.js`

**File:** `.gemini/skills/graphdb/scripts/query_graph.js`

**Goal:** Create a unified `hybrid-context` command.

1.  **Import Vector Search Logic**: Integrate the search query from the prototype `find_implicit_links.js`.
2.  **New Command**: `hybrid-context --function <name>`
    *   **Step 1 (Structure):** Fetch callers/callees (existing `test-context` logic).
    *   **Step 2 (Semantic):** Fetch top 5 vector matches for the target function (excluding the function itself).
    *   **Step 3 (Merge)::** Return a JSON object containing `structural_dependencies` and `semantic_related`.
    *   **Step 4 (Cluster):** Support "Cluster Analysis" to find cohesive groups of functions (Seams) for extraction. (See [plans/cluster_plan.md](./cluster_plan.md)).
3.  **Refinement:** Ensure the output distinguishes between "Definite Call" (Hard) and "Potential Clone/Relation" (Soft).

### Phase 5: Handling Stale Graphs
See dedicated plan: [plans/avoid_stale_graph.md](./avoid_stale_graph.md) for the "Git-Driven Delta" strategy to keep embeddings in sync during refactoring.

---

## 5. Documentation & Agent Interaction

### Agent Interaction Flow
This new capability allows the agent to build a **Refactoring Context Window**.

1.  **Trigger:** User asks to "Refactor `TargetFunction` logic".
2.  **Tool Selection:** Agent uses `query_graph.js hybrid-context --function TargetFunction`.
3.  **Output Parsing:** The tool returns:
    *   **Hard Dependencies:** `TargetFunction` calls `HelperFunction`.
    *   **Soft Dependencies:** `TargetFunction` is 98% similar to `TargetFunctionLegacy` (Potential clone!).
4.  **Action:** Agent reads the source of *all* these functions to ensure the refactor handles the legacy clone and the active logic simultaneously.

### `SKILL.md` Updates
*   **Frontmatter:** Update `description` to include "semantic search and implicit dependency discovery".
*   **Tool Usage:** Document the `hybrid-context` command in `query_graph.js`.
*   **Instruction:** "Use `hybrid-context` when preparing to refactor a function to ensure you catch hidden dependencies and logical clones."

### `README.md`
*   Add section **"Vector Search Support"**.
*   Document the `enrich_vectors.js` script in the **Ingestion Pipeline** section.
*   List new env vars: `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION`.

---

## 6. Verification Checklist

1.  [ ] Unit tests for `VectorService` pass (mocked ADC, mocked 429 errors).
2.  [ ] `enrich_vectors.js` runs on the target graph without errors and respects rate limits.
3.  [ ] Neo4j Browser shows `embedding` property populated.
4.  [ ] `query_graph.js hybrid-context` returns both structural neighbors and semantic matches.