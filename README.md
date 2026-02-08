# GraphDB Skill Ecosystem

## 🤖 Multi-Agent Orchestration

This project uses a sophisticated multi-agent orchestration pattern to handle complex modernization tasks. 

### The Supervisor (System Prompt)
The orchestration is managed by a **Supervisor protocol** defined in `.gemini/system.md`. 
*   **Rationale:** We moved from a standalone `supervisor` agent to a `system.md` override to ensure the primary agent (you) has the native authority to dispatch specialized sub-agents (Scout, Engineer, etc.) without intermediate layers of delegation that can obscure context or restrict tool access.
*   **Enabling:** To activate this protocol, ensure your environment is configured with `GEMINI_SYSTEM_MD=true`.

### Specialized Sub-Agents
Each agent has a dedicated role and system prompt located in `.gemini/agents/`.

*   **Architect**: The planner. Manages the roadmap (`@plans/`), prioritizes technical debt, and creates detailed implementation plans.
*   **Scout**: The researcher. Uses the GraphDB to map dependencies, identify global state usage, and find architectural "seams" for refactoring.
*   **Engineer**: The builder. Implements changes using strict Test-Driven Development (TDD). **Constraint:** Cannot perform git commits.
*   **Auditor**: The gatekeeper. Verifies quality standards and passes tests. **Constraint:** Cannot perform git commits.
*   **MSBuild**: The specialist. Handles running builds and tests, providing concise error reporting.

## 🛠️ Build & Ingestion Workflow

To analyze a codebase, you must first ingest it into the Graph Database. Run these commands from the **project root**:

1.  **Extract Graph Data** (Parses source code to JSON):
    ```bash
    node .gemini/skills/graphdb/extraction/extract_graph.js
    ```
2.  **Import to Neo4j** (Loads JSON into DB):
    ```bash
    node .gemini/skills/graphdb/scripts/import_to_neo4j.js
    ```
3.  **Enrichment** (Adds Git history & Semantic Vectors):
    ```bash
    node .gemini/skills/graphdb/scripts/analyze_git_history.js
    node .gemini/skills/graphdb/scripts/enrich_vectors.js
    ```

## 🔍 Usage & Analysis

The project follows a **"Search-Refine-Analyze"** workflow:

### 1. Discovery (Ripgrep)
Use standard `search_file_content` to find initial keywords and entry points.

### 2. Structural Analysis (GraphDB)
Use the CLI tools to understand deep dependencies.

*   **Find Seams (Decoupling Points):**
    ```bash
    node .gemini/skills/graphdb/scripts/query_graph.js seams --module <ModuleName>
    ```
*   **Analyze Test Context:**
    ```bash
    node .gemini/skills/graphdb/scripts/query_graph.js test-context --function <FunctionName>
    ```
*   **Check Hotspots:**
    ```bash
    node .gemini/skills/graphdb/scripts/query_graph.js hotspots --module <ModuleName>
    ```

### 3. Semantic Search (Implicit Links)
Find code by *meaning* rather than exact syntax (e.g., finding SQL strings that modify a table).

*   **Search:**
    ```bash
    node .gemini/skills/graphdb/scripts/find_implicit_links.js --query "natural language query"
    ```

## ⚡ Utilities (Neo4j Manager)

If you need to switch between different project databases (Neo4j Community limit):

*   **List Databases:**
    ```bash
    node .gemini/skills/neo4j-manager/scripts/list_databases.js
    ```
*   **Switch Database:**
    ```bash
    node .gemini/skills/neo4j-manager/scripts/switch_database.js <TargetDBName>
    ```

## 🕵️ Agent Execution Tracing

To understand the complex interactions between agents (e.g., CLI -> Supervisor -> Engineer), the project includes a configured execution tracer.

*   **Log File:** `.gemini/execution-trace.jsonl`
*   **Mechanism:** A hook script (`.gemini/hooks/agent-tracer.js`) intercepts `BeforeAgent`, `AfterAgent`, `BeforeTool`, and `AfterTool` events.
*   **Purpose:**
    *   Visualize the call stack of nested agents.
    *   Debug "Human in the Loop" interactions (e.g., does the stack unwind or pause?).
    *   Audit tool usage and arguments in real-time.

### 📊 Trace Viewer

A lightweight, single-file HTML viewer is included to visualize the trace logs.

1.  **Open:** Open `trace-viewer.html` in any modern browser.
2.  **Load:** Drag & Drop `.gemini/execution-trace.jsonl` onto the page.
3.  **Analyze:** Filter by session or file to see the chronological lineage of agent operations.

To disable tracing, remove the `hooks` section from `.gemini/settings.json`.
