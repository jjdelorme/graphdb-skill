# Plan: Snippet Generation & Usage Context (Lazy Search)

**Goal:** Enable the `graphdb` skill to return specific source code snippets for definitions and usage contexts without bloating the graph database or consuming excessive tokens by reading entire files.

**Strategy:** **"Lazy Search"** (Just-In-Time Retrieval).
Instead of storing every line of code or usage location in the Graph DB (which scales poorly), we use the Graph as an **Index** to point to the file system. The tools will perform targeted reads on demand.

---

## 1. Rationale

### The Problem
1.  **Missing "Usage" Context:** The current graph knows *that* Function A uses Global B, but not *where* (Line 42) or *how* (e.g., `B.MemberField`).
2.  **Token Bloat:** To find this context, the Agent currently has to read the entire file containing Function A, wasting context window and time.
3.  **Hidden Macros:** Static analysis might miss macro expansions. Reading the source text is the only way to verify "purity" for modernization.

### The Solution: Lazy Search
*   **Graph:** Stores "Truth" about relationships and definitions (`File`, `StartLine`, `EndLine`).
*   **Tool:** Uses this metadata to surgically extract only the relevant lines from the disk.

---

## 2. New Tooling Architecture

We will implement two specific tools to handle "Definition Inspection" and "Usage Location".

### Tool A: `fetch_node_source.js`
*   **Purpose:** Retrieve the definition of any node found in the graph.
*   **Input:** `NodeID` (preferred) or `Type:Name`.
*   **Logic:**
    1.  Query Neo4j: `MATCH (n) WHERE n.id = $id RETURN n.file, n.start_line, n.end_line`.
    2.  File I/O: Read `n.file`.
    3.  Slice: Return lines from `start_line` to `end_line`.
*   **Use Case:** Agent finds a function via Vector Search and wants to see its code immediately.

### Tool B: `locate_dependency_usage.js`
*   **Purpose:** Pinpoint exactly where a dependency is accessed *within* a function.
*   **Input:** `SourceNodeID` (The Caller/User), `TargetNodeID` (The Callee/Variable).
*   **Logic:**
    1.  Query Neo4j: Get Source's `file`, `start_line`, `end_line` AND Target's `name`.
    2.  File I/O: Read Source's text (bounded by lines).
    3.  **Context Scan:** Perform a Regex/Text search within that window for Target's name.
    4.  **Output:** Return matching lines with context (e.g., +/- 2 lines).
*   **Use Case:** "Show me how `UpdateStock` uses the global `InventoryTable`." -> Returns 3 lines showing the SQL query construction.

---

## 3. Integration with Existing Workflows

### 3.1 Vector Search Integration (`hybrid-context`)
The `fetch_node_source` tool completes the Vector Search workflow:
1.  **Search:** `find_implicit_links.js --query "calculates tax"` -> Returns Node `n123`.
2.  **Inspect:** `fetch_node_source.js --id n123` -> Returns the actual code.

### 3.2 Modernization Workflow
This replaces the "Manual Verify" step with a tool-assisted one:
1.  **Analyze:** `query_graph.js seams --module X` -> Lists dependencies.
2.  **Verify:** `locate_dependency_usage.js --source X --target Y` -> Confirms the nature of the dependency (Read vs Write, Member Access).

---

## 4. Implementation Plan

### Phase 1: Core Logic (TDD)
**File:** `.gemini/skills/graphdb/scripts/tools/SnippetService.js` (New)
1.  **Function `sliceFile(path, start, end)`**:
    *   Efficiently reads specific lines.
    *   Handles "File Not Found" gracefully.
2.  **Function `findPatternInScope(fileContent, pattern, contextLines)`**:
    *   Searches for a string/regex within a text block.
    *   Returns formatted snippet with line numbers.
3.  **Test:** Create `tests/SnippetService.test.js` to verify off-by-one errors and context windowing.

### Phase 2: CLI Wrappers
**Files:**
*   `.gemini/skills/graphdb/scripts/fetch_source.js`: Wraps `SnippetService.sliceFile` + Neo4j lookup.
*   `.gemini/skills/graphdb/scripts/locate_usage.js`: Wraps `SnippetService.findPatternInScope` + Neo4j lookup.

### Phase 3: Documentation & Registration
1.  **Update `SKILL.md`**:
    *   Add new tools to the list.
    *   Add a "Usage Example" showing the `Query -> Locate -> Refactor` loop.
2.  **Update `package.json`**: Ensure any new dependencies (minimal expected) are saved.

---

## 5. Verification
*   **Unit Test:** Verify `SnippetService` correctly extracts lines 10-15 from a dummy file.
*   **Integration Test:** Run `locate_usage` on a known relationship (e.g., `main` calls `helper`) and verify it returns the correct line of the call.
