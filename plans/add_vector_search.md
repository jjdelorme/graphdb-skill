# Plan: Hybrid Graph with Vector Search (GraphRAG)

**Goal:** Enhance the existing Code Property Graph (CPG) with semantic vector embeddings to identify implicit dependencies (e.g., cross-language calls, string-based APIs) that static analysis misses.

**Target Model:** `gemini-embedding-001` via Vertex AI.
**Library:** `google.genai` (Node.js SDK) or Google Cloud Vertex AI REST API.
**Authentication:** **Vertex AI ONLY** (Google Application Default Credentials - ADC).
**Database:** Neo4j Community Edition (v5.15+ required for Vector Indexes).

---

## 1. Architecture Update

We will move from a pure **Structural Graph** to a **Hybrid Graph**.

### New Components
1.  **`VectorService`**: A Node.js module interacting with Google Vertex AI.
    *   **CRITICAL:** Must implement **Exponential Backoff** and **Retry Logic** to handle HTTP 429 (Quota Exceeded) errors robustly.
2.  **`enrich_vectors.js`**: A standalone script that iterates existing `Function` nodes, reads their source code from disk, generates embeddings, and updates the graph.
3.  **`find_implicit_links.js`**: A new CLI tool that accepts a natural language query (or code snippet), vector searches the graph, and proposes "Soft Links".

### Schema Changes (Neo4j)
*   **Node Property**: `Function.embedding` (Vector<Float>, 768 dimensions for `gemini-embedding-001`).
*   **Index**: `function_embeddings` (Vector Index).

---

## 2. Test-First Strategy (TDD)

We will harden the existing prototype code by adding missing test coverage before refactoring.

### A. Update `VectorService.test.js`
*   **Existing Tests**: Configuration, Embedding Generation, Batching.
*   **New Test (Critical)**: **Rate Limit Handling**.
    *   Mock a 429 error from the API.
    *   Assert the service waits (backoff) and retries.
    *   Assert it eventually succeeds or fails after Max Retries.

### B. Update `EnrichmentLogic.test.js` (or create if missing)
*   **Test 1: Source Extraction**: Verify it respects the `start_line` / `end_line` from the graph.
*   **Test 2: Safety Filter**: Verify the logic (or Cypher) explicitly excludes `node_modules`.

---

## 3. Implementation Plan

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

### Phase 4: Query Tool Refinement

**File:** `.gemini/skills/graphdb/scripts/find_implicit_links.js`

**Usage:** `node find_implicit_links.js --query "updates inventory table"`

**Logic:**
1.  Generate embedding for the query string.
2.  Cypher:
    ```cypher
    CALL db.index.vector.queryNodes('function_embeddings', 10, $queryVector)
    YIELD node, score
    RETURN node.name, node.file, score
    ```

---

## 4. Documentation & Agent Interaction

### Agent Interaction Flow
This new capability allows the agent to bridge the gap between "User Intent" and "Code Implementation".

1.  **Trigger:** User asks a vague discovery question (e.g., *"Where is the billing logic?"* or *"How do we handle PDF export?"*).
2.  **Tool Selection:** Agent identifies `find_implicit_links.js` from `SKILL.md` as the discovery tool.
3.  **Execution:** Agent runs `node .gemini/skills/graphdb/scripts/find_implicit_links.js --query "billing logic"`.
4.  **Output Parsing:** The tool returns a JSON array of candidates (`{ name, file, score }`).
5.  **Refinement (Optional):** The agent *may* autonomously follow up with `query_graph.js test-context --function <name>` on the top result to verify its relevance before presenting it to the user.

### `SKILL.md` Updates
*   **Frontmatter:** Update `description` to include "semantic search and implicit dependency discovery".
*   **Tool Usage:** Document `find_implicit_links.js` explicitly.
*   **Output Format:** Specify that the tool returns **JSON**.
*   **Instruction:** "Use this tool when you cannot find a function by exact name, or when the user describes *behavior* rather than *syntax*."

### `README.md`
*   Add section **"Vector Search Support"**.
*   Document the `enrich_vectors.js` script in the **Ingestion Pipeline** section.
*   List new env vars: `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION`.

---

## 5. Verification Checklist

1.  [ ] Unit tests for `VectorService` pass (mocked ADC, mocked 429 errors).
2.  [ ] `enrich_vectors.js` runs on the "Alpine" graph without errors and respects rate limits.
3.  [ ] Neo4j Browser shows `embedding` property populated.
4.  [ ] `find_implicit_links.js` returns relevant functions for a vague query (e.g., "draws the balcony").
