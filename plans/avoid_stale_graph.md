# Plan: Incremental Graph Updates (Avoid Stale Data)

**Goal:** Ensure the Code Property Graph (CPG) and Vector Embeddings remain synchronized with the codebase as the user (or agent) modifies code, without requiring expensive full re-ingestion.

**Philosophy:** "Fast Deltas over Full Rebuilds." reliability is maintained by leveraging **Git** as the source of truth for change detection.

---

## 1. The "Stale Graph" Problem
Refactoring is a loop: `Analyze -> Plan -> Edit -> Verify`.
1.  **The Risk:** After the "Edit" step, line numbers shift, functions are renamed, and logic changes.
2.  **The Impact:**
    *   **Hallucination:** The agent reads line 50-60 based on the graph, but the function is now at 55-65.
    *   **Bad Embeddings:** The vector represents the *old* logic, causing the agent to miss semantically relevant connections to the *new* logic.

## 2. Architecture: Git-Driven Deltas

We will introduce a state tracking mechanism to detect "Drift".

### Data Model Changes (Neo4j)
*   **New Node:** `(:GraphState { last_indexed_commit: "sha1...", updated_at: timestamp })`
*   **Logic:**
    *   On any tool execution, compare `GraphState.last_indexed_commit` vs `git rev-parse HEAD`.
    *   If they differ, calculate the Delta.

### New Script: `scripts/sync_graph.js`
This script handles the "Re-sync" logic.

#### Step 1: Detect Changed Files
Use Git to find files changed *since the last sync*.
```bash
git diff --name-only <last_indexed_commit> HEAD
```
*   **Filter:** Apply the same ignores as the main ingestion (no `node_modules`, `obj/`, etc.).

#### Step 2: Strategy Selection (Thresholds)
*   **Micro-Change (< 5 files):** **Synchronous Update**. Run immediately before answering the user's question. Latency ~2-5s.
*   **Macro-Change (> 5 files):** **Asynchronous / User Prompt**. Warn the user: *"Graph is 12 commits behind. Run /sync to update."*

#### Step 3: File-Level Re-Ingestion (The "Surgical" Fix)
For each changed file `F`:
1.  **Delete Old Data:**
    *   Cypher: `MATCH (f:File {path: $path}) DETACH DELETE f` (Cascades to functions contained in it? *Careful: We need to preserve the ID stability if possible, or just nuke and rebuild the file's subgraph.* Rebuilding the file's subgraph is safer for consistency).
2.  **Re-Parse:**
    *   Run `TreeSitter` on the new file content.
3.  **Re-Embed (Optimized):**
    *   **Hash Check:** Calculate `MD5(function_body)`.
    *   Compare with a local cache (or property on the old node before deleting).
    *   **Cache Hit:** Re-use existing vector (Save $$$ and Time).
    *   **Cache Miss:** Call Vertex AI API for new embedding.
4.  **Link:**
    *   Re-establish `CALLS` edges. (Note: This might require a "Linker" pass if the function calls *other* files, but for a localized update, we primarily care about *outgoing* calls from the new code).

---

## 3. Implementation Stages

### Stage 1: State Tracking
*   Modify `extract_graph.js` to write the current `HEAD` commit to Neo4j at the end of a run.

### Stage 2: The `delta` Tool
*   Create `scripts/analyze_git_history.js` (or use existing) to expose a function `getChangedFiles(sinceCommit)`.

### Stage 3: The Surgical Updater (`update_file.js`)
*   Extract the "Single File Parsing" logic from `GraphBuilder.js` so it can be called on demand.
*   Implement the `Delete -> Parse -> Embed -> Insert` flow for a single path.

### Stage 4: Agent Integration
*   **Hook:** Inside `query_graph.js`, adds a pre-flight check.
*   **Output:** If a Delta is detected and auto-patched, append a footer to the agent's response: `(Graph synced: 3 files updated)`.

---

## 4. Risks & Mitigations

*   **Risk:** "Dangling Edges". If `File A` calls `File B`, and we delete `File A` to rebuild it, the edge is gone. When we rebuild `File A`, we must ensure we can find `File B` again to reconnect.
    *   *Mitigation:* The `GraphBuilder` logic relies on looking up nodes by `Type:Name`. Since `File B` is still in the graph, the standard "Pass 2" logic should find it successfully.
*   **Risk:** API Costs.
    *   *Mitigation:* The **MD5 Hash Check** is critical. If I only change a comment, the code vector shouldn't change.