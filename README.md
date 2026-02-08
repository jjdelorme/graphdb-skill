# GraphDB Skill Ecosystem

## 🤖 Multi-Agent Orchestration

This project uses a sophisticated multi-agent orchestration pattern to handle complex modernization tasks. The system is designed to be **self-correcting**, ensuring that plans match reality before code is committed.

### The Protocol Lifecycle

The system moves through distinct phases, enforced by the Supervisor. The **"Three-Way Decision Fork"** in the verification phase ensures plans remain realistic.

```mermaid
graph TD
    %% Roles
    subgraph Phase 1 & 2: Strategy
        Scout[Scout: Research & Map]
        Architect[Architect: Plan & Strategy]
    end

    subgraph Phase 3: Construction
        Engineer[Engineer: Implement]
        Auditor[Auditor: Verify]
    end

    %% Flow
    Start([User Start]) --> Scout
    Scout --> Architect
    Architect --> Review{User Approval}
    
    Review -- Reject --> Architect
    Review -- Approve --> Engineer
    
    Engineer --> Auditor
    
    %% The Three-Way Fork
    Auditor -- Code Broken? --> Engineer
    Auditor -- Plan Wrong? --> Architect
    Auditor -- Verified --> Commit([Git Commit])
    
    Commit --> Engineer
```

### The Supervisor (System Prompt)
The orchestration is managed by a **Supervisor protocol** defined in `.gemini/system.md`. 
*   **Rationale:** We moved from a standalone `supervisor` agent to a `system.md` override to ensure the primary agent (you) has the native authority to dispatch specialized sub-agents (Scout, Engineer, etc.) without intermediate layers of delegation that can obscure context or restrict tool access.
*   **Enabling:** To activate this protocol, ensure your environment is configured with `GEMINI_SYSTEM_MD=true`.

### Specialized Sub-Agents
Each agent has a dedicated role and system prompt located in `.gemini/agents/`.

*   **Architect**: The **Planner**. Manages the roadmap (`@plans/`) and creates detailed implementation plans. Called when plans are missing or need correction (reality check).
*   **Scout**: The **Researcher**. Uses the GraphDB to map dependencies, identify global state usage, and find architectural "seams".
*   **Engineer**: The **Builder**. Implements changes using strict Test-Driven Development (TDD). **Constraint:** Cannot perform git commits.
*   **Auditor**: The **Quality & Consistency Gatekeeper**. Verifies tests and ensures the active Plan matches the Codebase reality.
    *   *Code Failure:* Dispatches `engineer` to retry.
    *   *Plan Failure:* Dispatches `architect` to fix the plan.
    *   *Success:* Allows the Supervisor to proceed to Git Commit.
*   **MSBuild**: The **Specialist**. Handles running builds and tests, providing concise error reporting.

## 🗄️ Neo4j Database Setup

This project uses a Neo4j database containerized with Podman/Docker. A helper script is provided to launch the container with the correct APOC plugins and configuration.

1.  **Launch Container:**
    ```bash
    bash .gemini/skills/neo4j-manager/scripts/start_neo4j_container.sh
    ```
    *   **User:** `neo4j`
    *   **Password:** `password`
    *   **UI:** [http://localhost:7474](http://localhost:7474)

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

The project follows a **"Graph-First"** workflow powered by the **`graphdb` skill**. This specialized skill provides a unified interface for both structural (Neo4j) and semantic (Vector Embeddings) analysis.

### 1. Structural & Semantic Analysis (The `graphdb` Skill - PRIMARY)
**ALWAYS START HERE.** You must activate the `graphdb` skill (`activate_skill(name="graphdb")`) to access its specialized investigative capabilities.

*   **Structural Queries (Relationships):** Map inheritance, function calls, and global state.
    ```bash
    node .gemini/skills/graphdb/scripts/query_graph.js <command>
    ```
*   **Semantic Queries (Vectors):** Find code by *meaning* rather than syntax using the skill's vector search capabilities.
    ```bash
    node .gemini/skills/graphdb/scripts/find_implicit_links.js --query "business logic for..."
    ```

### 2. Text Search (Fallback)
Use standard `search_file_content` (Ripgrep) **ONLY** when the `graphdb` skill cannot provide the necessary data (e.g., searching for non-code assets or literal TODOs).

**⚠️ WARNING:** Relying on grep for dependency or architectural analysis is prohibited. The `graphdb` skill is the source of truth for code relationships.

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

1.  **Start Server:** (Optional but recommended for browser compatibility)
    ```bash
    # Option A: Node.js
    npx http-server .
    # Option B: Python
    python3 -m http.server 8080
    ```
2.  **Open:** Navigate to `http://localhost:8080/trace-viewer.html` (or open the file directly in a modern browser).
3.  **Load:** Drag & Drop `.gemini/execution-trace.jsonl` onto the page.
4.  **Analyze:** Filter by session or file to see the chronological lineage of agent operations.

To disable tracing, remove the `hooks` section from `.gemini/settings.json`.
