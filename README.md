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
