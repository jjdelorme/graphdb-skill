---
name: graphdb
description: Expert in analyzing project architecture using a Neo4j Code Property Graph (CPG) enhanced with Vector Search. Answers questions about dependencies, seams, testing contexts, implicit links, and risks.
---

# Graph Database Skill

You are an expert in analyzing the project's architecture using the Code Property Graph (CPG).
Your goal is to answer questions about dependencies, seams, testing contexts, and architectural risks.

## Tool Usage
You will use the `query_graph.js` CLI tool to fetch data.
**Base Command:** `node .gemini/skills/graphdb/scripts/query_graph.js <query_type> [options]`

## Setup & Infrastructure

The graph database is built in two stages: extraction and import.

### 1. Extraction
Scans the codebase and generates intermediate JSON files (`nodes.json`, `edges.json`).
*   **Command:** `node .gemini/skills/graphdb/extraction/extract_graph.js`
*   **Supported Extensions:** C/C++, C#, VB.NET, ASP.NET, SQL.
*   **Zero-Config:** Automatically scans the project root and respects `.gitignore`.

### 2. Import
Loads the JSON files into a Neo4j database.
*   **Prerequisites:** A running Neo4j instance. Configure credentials in `.env`:
    ```
    NEO4J_URI=bolt://localhost:7687
    NEO4J_USER=neo4j
    NEO4J_PASSWORD=your_password
    ```
*   **Command:** `node .gemini/skills/graphdb/scripts/import_to_neo4j.js`

## Primary Use Cases

### 1. Identifying Seams (Decoupling Points)
Find the best functions to extract to break dependencies.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js seams --module <module_name>`
*   **Output Analysis:** Look for functions with high "Incoming" count but low "Outgoing" count.

### 2. Analyzing Dependencies (Test Context)
Determine what a function depends on (globals, other functions) to set up a test harness.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js test-context --function <function_name>`
*   **Output Analysis:** Detailed list of global variables read/written and function calls.

### 3. UI Contamination Check
Check if a function or module is "contaminated" by UI dependencies.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js ui-contamination --function <function_name>`
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js ui-contamination --module <module_name>`

### 4. Risk & Hotspots
Find functions with high complexity and frequent changes.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js hotspots --module <module_name>`

### 5. Semantic Search (Implicit Links)
Find code based on semantic meaning (what it does) rather than exact syntax. Useful for finding hidden dependencies or loose coupling (e.g., cross-language calls, string-based APIs).
*   **Command:** `node .gemini/skills/graphdb/scripts/find_implicit_links.js --query "<natural language query>"`
*   **Example:** `node .gemini/skills/graphdb/scripts/find_implicit_links.js --query "updates inventory table"`
*   **Output Analysis:** Returns a list of functions sorted by semantic similarity score.

### 6. Hybrid Context (Refactoring Helper)
Prepare for refactoring by finding both explicit structural dependencies and implicit semantic clones/relations.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js hybrid-context --function <function_name>`
*   **Output Analysis:** Returns `structural_dependencies` (hard links) and `semantic_related` (potential clones/logical groupings).

### 7. Code Inspection
Retrieve actual source code or pinpoint usage locations using graph metadata.

**Fetch Definition:** Get the source code for a specific node.
*   **Command:** `node .gemini/skills/graphdb/scripts/fetch_source.js --id <NodeID>`
*   **Use Case:** Inspecting the code of a function found via Vector Search.

**Locate Usage:** Find exactly where a dependency is used within a function.
*   **Command:** `node .gemini/skills/graphdb/scripts/locate_usage.js --source <CallerID> --target <CalleeID>`
*   **Use Case:** Identifying exact lines where a global variable is modified.

## Other Available Query Types
*   `globals`: List global variable usage.
*   `extract-service`: Suggest functions for service extraction.
*   `impact`: Analyze impact of changes (reverse dependencies).
*   `co-change`: Find files that frequently change together.
*   `progress`: Check refactoring progress stats.
*   `function`: Get raw metadata for a function.

## Operational Guidelines
*   **Output:** The tool returns JSON. You should parse this and present a concise, readable summary (bullet points, tables).
*   **Context:** Always mention the source file and line number when discussing a function.
*   **Missing Data:** If a query returns empty, verify the spelling of the function/module name.
