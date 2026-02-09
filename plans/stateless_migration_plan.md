# Stateless GraphDB Migration Plan

## 🎯 Objective
Migrate the local `graphdb-skill` from an in-memory, stateful architecture to the robust **stateless, streaming architecture** found in the `alpine/VIEW` reference.

**Crucial Constraint:** We must **preserve** the local project's extended language support (TypeScript, VB.NET, ASP.NET) and newer `tree-sitter` dependencies (`^0.22.4`), which were removed or downgraded in the `alpine` version.

## 📦 Phase 1: Core Architecture Refactoring

### 1. Update `GraphBuilder.js` (The Heart of the Change)
Convert `extraction/core/GraphBuilder.js` to emit JSONL (JSON Lines) streams immediately instead of holding arrays in memory.

*   **Remove State:** Delete `this.nodes`, `this.edges`, `this.nodeMap`, and `this.nodeIdCounter`.
*   **Add Streaming:** In `constructor`, initialize `fs.createWriteStream` for `nodes.jsonl` and `edges.jsonl`.
*   **Deterministic IDs:** Implement `_generateId(type, name)` using `crypto.createHash('md5')`.
*   **Update `getNode`:**
    *   Generate deterministic ID.
    *   Construct node object.
    *   **Write immediately** to `this.nodesStream`.
    *   Return ID.
*   **Update `addEdge`:**
    *   Construct edge object.
    *   **Write immediately** to `this.edgesStream`.
*   **Memory Management:** Add `global.gc()` calls inside the main loop (every ~500 items).
*   **Logic Porting:**
    *   Copy `ImplicitGlobalWrite` handling (emitting `inferred: true` nodes).
    *   Copy `MFC_API` heuristic logic.
    *   Ensure base classes in `INHERITS_FROM` are emitted as skeleton nodes to prevent dangling edges.

### 2. Update `extract_graph.js` (Orchestration)
Update the entry point to support surgical updates while keeping all local adapters.

*   **CLI Arguments:** Add parsing for `--file-list <path>` to allow extracting a subset of files (crucial for surgical updates).
*   **Adapter Preservation:** Ensure `TsAdapter`, `VbAdapter`, and `AspAdapter` remain instantiated and passed to `GraphBuilder`. **Do not** comment them out like in the `alpine` reference.
*   **File Scanning:** Retain the glob patterns for `.ts`, `.tsx`, `.vb`, `.asp`, etc.

### 3. Update `import_to_neo4j.js` (Ingestion)
Refactor the import script to handle streaming JSONL data and database-side deduplication.

*   **Switch to Streams:** Use `fs.createReadStream` and `readline` interface to process `nodes.jsonl` and `edges.jsonl` line-by-line.
*   **Batching:** Accumulate lines into batches (e.g., 2000 items) before sending to Neo4j.
*   **Deduplication:** Update Cypher queries to use `MERGE` instead of `CREATE` for nodes, as the stateless builder will emit the same node multiple times (once for every file it appears in).
    *   *Note:* `edges` generally use `MERGE` as well to avoid duplicates if the same relationship is scanned twice.

## 🛠️ Phase 2: Tooling & Reliability

### 4. Port Utility Scripts
Copy valuable operational scripts from `alpine` that are missing locally.

*   **`scripts/manage_embeddings.js`:** For backing up/restoring vector embeddings (critical for cost savings).
*   **`scripts/debug_enrichment.js`:** Helper for debugging vector enrichment issues.
*   **`scripts/graph_status_report.js`:** For quick stats on the graph.
*   **`scripts/orchestrate.js`:** Likely a higher-level runner; review and port if useful for automation.
*   **`scripts/run_restore_sequence.js`:** Automation for the restore workflow.

### 5. Test & Validation
*   **Port Tests:** Copy `extraction/test/ImplicitGlobal.test.js` to local.
*   **Validation:** Run `extract_graph.js` on the local repository itself to ensure:
    *   JSONL files are created.
    *   Memory usage stays low.
    *   TypeScript/VB files are still processed correctly.

## ⚠️ Risks & Mitigations

*   **Tree-sitter Compatibility:** The `alpine` version downgraded `tree-sitter`. We are keeping the newer version.
    *   *Mitigation:* If "Illegal Instruction" or WASM errors occur, we may need to rebuild `tree-sitter-wasms` or strictly pin versions. We will attempt to run with current versions first.
*   **Duplicate Nodes:** Stateless emission produces massive duplication in the JSONL files.
    *   *Mitigation:* The `import_to_neo4j.js` refactor using `MERGE` is non-negotiable. It shifts the deduplication load to the database.

## 📝 Execution Checklist

- [ ] Modify `extraction/core/GraphBuilder.js`
- [ ] Modify `extraction/extract_graph.js`
- [ ] Modify `scripts/import_to_neo4j.js`
- [ ] Create `scripts/manage_embeddings.js`
- [ ] Create `scripts/debug_enrichment.js`
- [ ] Create `scripts/graph_status_report.js`
- [ ] Create `scripts/orchestrate.js`
- [ ] Create `extraction/test/ImplicitGlobal.test.js`
- [ ] Verify `package.json` dependencies (ensure `crypto` is available - built-in to Node).
