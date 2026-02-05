# Modernization Workflow: The Synergy of Agent, Ripgrep, and GraphDB

This document outlines the ideal workflow for modernizing complex, legacy applications (e.g., monoliths to microservices, language migrations) by leveraging the distinct strengths of the Gemini CLI Agent, `ripgrep`, and the `graphdb` skill.

## The Triad of Modernization

We employ a "Search-Refine-Analyze" strategy where each tool covers a specific layer of code understanding.

| Component | Role | Capability | Best For |
| :--- | :--- | :--- | :--- |
| **1. Gemini Agent** | **Orchestrator** | Logic, Planning, Code Editing | Synthesizing information, making decisions, and executing changes. |
| **2. Ripgrep (`rg`)** | **Tactical Search** | High-speed Lexical Search | Finding exact symbol usage, string literals, and localizing patterns instantly. |
| **3. GraphDB Skill** | **Strategic Analysis** | Structural & Semantic Analysis | Understanding dependencies, "seams," implicit links (vectors), and global architecture. |

---

## The Workflow: Modernizing a Legacy Feature

### Phase 1: Discovery (Broad & Fast)
**Goal:** Locate the entry points and general footprint of the feature.

1.  **Agent Action:** The user asks, *"I need to extract the 'Inventory Update' logic into a service."*
2.  **Tool:** `ripgrep` (via `search_file_content`)
    *   **Query:** "Inventory", "UpdateStock", "tbl_inv"
    *   **Outcome:** Returns a list of 50+ file matches.
3.  **Result:** The Agent identifies that `InventoryModule.vb` and `StockController.cs` are the primary hotspots.
    *   *Limitation:* `ripgrep` cannot tell us if `StockController` *calls* `InventoryModule` or if they just share a naming convention.

### Phase 2: Structural Assessment (Deep & Relational)
**Goal:** Understand the "blast radius," dependencies, and how to safely extract the code.

1.  **Agent Action:** *"Analyze the dependencies of `InventoryModule` to see how coupled it is."*
2.  **Tool:** `graphdb` (Skill)
    *   **Query:** `node .../query_graph.js seams --module InventoryModule.vb`
    *   **Query:** `node .../query_graph.js test-context --function UpdateStock`
    *   **Outcome:** The Graph reveals:
        *   `UpdateStock` reads 3 global variables (High coupling).
        *   It is called by 12 different UI event handlers (High impact).
        *   It has a low "Outgoing" count (Good candidate for a seam).

### Phase 3: Semantic "Ghost Hunting" (Implicit & Conceptual)
**Goal:** Find hidden dependencies that static analysis (graph) and exact match (ripgrep) miss. This includes SQL strings, reflection, or poorly named functions.

1.  **Agent Action:** *"Are there any other places that modify inventory that don't call this function directly?"*
2.  **Tool:** `graphdb` (Vector Search)
    *   **Query:** `node .../find_implicit_links.js --query "decrements product quantity"`
    *   **Outcome:** Returns `LegacyOrderScript.asp`.
3.  **Insight:** The Agent discovers a hidden ASP script that executes a raw SQL `UPDATE` statement on the inventory table, bypassing the `InventoryModule`.
    *   *Critical Value:* **This prevents a bug where the new microservice would be out of sync with legacy direct-DB writes.**

### Phase 4: Execution & Verification
**Goal:** Refactor the code with confidence.

1.  **Agent Action:** *"Isolate `UpdateStock`, refactor the global variables into parameters, and fix the raw SQL in the ASP file."*
2.  **Tools:**
    *   `read_file` to ingest the specific files identified in Phases 2 & 3.
    *   `replace` or `write_file` to perform the refactoring.
    *   `ripgrep` to verify all references to the old global variables are gone.
3.  **Agent Action:** *"Verify the architectural improvement."*
4.  **Tool:** `graphdb`
    *   **Query:** `node .../query_graph.js seams --module InventoryModule.vb`
    *   **Outcome:** Confirms that "Incoming" links are preserved (via the new interface) but "Global Variable" dependencies are now 0.

## Summary of Responsibilities

*   **Ripgrep** is the **Eyes**: It quickly spots *where* things are. It is the first line of defense against unknown codebases.
*   **GraphDB** is the **Map**: It tells you *how* things are connected (Graph) and *what* things mean (Vectors). It prevents architectural violations.
*   **Agent** is the **Architect**: It uses the Eyes to look around and the Map to plan the route, ensuring the modernization journey doesn't break the application.
