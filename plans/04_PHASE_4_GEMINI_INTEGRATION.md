# Feature Implementation Plan: Phase 4 - Gemini CLI Skill Integration

**Campaign:** Gemini CLI Skill Integration (Campaign 3)
**Goal:** Complete the modernization by wrapping the new Go binary (`.gemini/skills/graphdb/scripts/graphdb`) in the existing Gemini Skill interface. This allows Agents to use the high-performance Go engine without changing their behavior or tool definitions.
**Context:** Currently, the skill relies on a suite of Node.js scripts (`extract_graph.js`, `query_graph.js`, etc.) with heavy dependencies (Tree-sitter, Neo4j Driver). We will replace the internals of these scripts to delegate work to the `graphdb` binary, effectively acting as a shim.

## 📋 Todo Checklist
- [ ] **Verification:** Map existing JS arguments to Go CLI flags.
- [ ] **Wrapper:** Update `extraction/extract_graph.js` to call `graphdb ingest`.
- [ ] **Wrapper:** Update `scripts/query_graph.js` to call `graphdb query`.
- [ ] **Wrapper:** Update `scripts/find_implicit_links.js` to call `graphdb query --type=search-similar`.
- [ ] **Deprecation:** Redirect/Deprecate `scripts/enrich_vectors.js` (enrichment is now part of ingest/enrich-features).
- [ ] **Cleanup:** Remove heavy NPM dependencies (`tree-sitter`, `neo4j-driver`) from `package.json`.
- [ ] **Documentation:** Update `SKILL.md` to reference the Go binary capabilities.
- [ ] **Testing:** Verify end-to-end `orchestrate.js` flow.

## 🔍 Analysis & Investigation

### The "Strangler Fig" Strategy
We will not change the file structure or the names of the scripts immediately. This ensures that:
1.  Existing Agents (which may have hardcoded paths) continue to work.
2.  `orchestrate.js` continues to work.
3.  We can rollback simply by reverting the JS file changes (and `npm install`).

### Mapping Requirements

#### 1. Extraction (`extract_graph.js`)
*   **Legacy:** `node extract_graph.js [path]`
*   **Target:** `.gemini/skills/graphdb/scripts/graphdb ingest [path]`
*   **Note:** The Go Ingestor combines Extraction + Enrichment.
    *   *Risk:* Enrichment cost.
    *   *Action:* Check if `graphdb ingest` has flags to control enrichment. If not, document this behavior change.

#### 2. Query (`query_graph.js`)
*   **Legacy:** `node query_graph.js <type> --target <target> ...`
*   **Target:** `.gemini/skills/graphdb/scripts/graphdb query --type <type> --target <target> ...`
*   **Mappings:**
    *   `hybrid-context` -> `graphdb query --type=hybrid-context`
    *   `test-context` -> `graphdb query --type=test-context` (or `neighbors`)
    *   `impact` -> `graphdb query --type=impact`
    *   `globals` -> `graphdb query --type=globals`
    *   `suggest-seams` -> `graphdb query --type=seams`

#### 3. Implicit Links (`find_implicit_links.js`)
*   **Legacy:** `node find_implicit_links.js --query "text"`
*   **Target:** `graphdb query --type=search-similar --target "text"`
*   **Gap Analysis:** We need to ensure the Go binary supports a raw "Search" type (Campaign 2 implemented `SearchFeatures` in the provider, we need to ensure the CLI exposes it).
    *   *Updated Logic:* `search-similar` maps to `SearchSimilarFunctions` (Dependency Layer). `search-features` maps to `SearchFeatures` (Intent Layer). The legacy `find_implicit_links.js` was primarily searching for functions, so `search-similar` is the correct mapping.

#### 4. Dependency Cleanup
*   The `package.json` currently includes `tree-sitter`, `neo4j-driver`, `openai` (or `vertex`).
*   The Wrapper scripts will only need `child_process` (Node.js built-in).
*   **Action:** `npm uninstall ...` to drastically reduce install time and size.

## 📝 Implementation Plan

### Prerequisites
*   `.gemini/skills/graphdb/scripts/graphdb` must be built and executable.
*   Go Ingestor and Query Engine (Campaign 1 & 2) must be complete (verified).

### Step-by-Step Implementation

#### Phase 4.1: The Wrappers
1.  **Step 4.1.A (Verification):** Verify CLI Help.
    *   *Action:* Run `.gemini/skills/graphdb/scripts/graphdb --help`, `.gemini/skills/graphdb/scripts/graphdb ingest --help`, `.gemini/skills/graphdb/scripts/graphdb query --help`, `.gemini/skills/graphdb/scripts/graphdb enrich-features --help`.
    *   *Goal:* Confirm flag names match our assumptions.
2.  **Step 4.1.B (Extraction):** Shim `extract_graph.js`.
    *   *Action:* Replace content of `extraction/extract_graph.js`.
    *   *Code:*
        ```javascript
        const { execSync } = require('child_process');
        // Resolve path to .gemini/skills/graphdb/scripts/graphdb
        // execSync(`${binPath} ingest ...`, { stdio: 'inherit' });
        ```
    *   *Test:* Run `node extraction/extract_graph.js` on a small folder. Verify `nodes.jsonl` is created.
3.  **Step 4.1.C (Query):** Shim `query_graph.js`.
    *   *Action:* Replace content of `scripts/query_graph.js`.
    *   *Code:* Map arguments to Go flags. Capture stdout. Parse JSON. Print result.
    *   *Test:* Run `node scripts/query_graph.js hybrid-context --function "Scan"`. Verify JSON output.

#### Phase 4.2: Enrichment & Search
1.  **Step 4.2.A (Enrichment):** Update `enrich_vectors.js`.
    *   *Action:* Modify it to print "Enrichment is now handled automatically during ingestion." and exit 0.
    *   *Rationale:* Don't break scripts that call it, but don't do double work.
2.  **Step 4.2.B (Search):** Shim `find_implicit_links.js`.
    *   *Action:* Map to `graphdb query --type=search-similar`.
    *   *Test:* Run `node scripts/find_implicit_links.js --query "parse"`.

#### Phase 4.3: Cleanup & Docs
1.  **Step 4.3.A (Dependencies):** Slim down `package.json`.
    *   *Action:* Remove `tree-sitter-*`, `neo4j-driver`, `glob`, `dotenv`, etc.
    *   *Keep:* `commander` (if used for parsing args in wrappers), or use `process.argv` directly if simple.
2.  **Step 4.3.B (Documentation):** Update `SKILL.md`.
    *   *Action:* Update "Tool Usage" section.
    *   *Action:* Add note about Go binary requirement.

### Testing Strategy
*   **Regression Test:** Use `scripts/orchestrate.js` to run a full cycle (Ingest -> Import -> Query).
*   **Output Comparison:** Ensure the JSON output from `query_graph.js` (wrapper) is identical in structure to the legacy output (Agent expects specific JSON fields).

## 🎯 Success Criteria
1.  **Agent Transparency:** The Agent can run its standard commands (`node .../query_graph.js ...`) and get correct results.
2.  **Performance:** "Ingest" is significantly faster (multithreaded Go).
3.  **Simplicity:** `package.json` has minimal/no dependencies.
4.  **Integration:** `orchestrate.js` runs without error.
