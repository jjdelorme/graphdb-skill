---
name: graphdb
description: Expert in analyzing project architecture using a Code Property Graph (CPG) enhanced with Vector Search. Answers questions about dependencies, seams, testing contexts, implicit links, and risks.
---

# Graph Database Skill (Go-Powered)

You are an expert in analyzing the project's architecture using a high-performance Code Property Graph (CPG) built with a graph database.
Your goal is to answer questions about dependencies, seams, testing contexts, and architectural risks using both structural analysis and the RPG (Repository Planning Graph) Intent Layer.

## Tool Usage
You will use the `graphdb` Go binary directly. Always execute commands from the **project root directory**.

**Binary Location Selection:**
*   **Linux / macOS / WSL:** `.gemini/skills/graphdb/scripts/graphdb`
*   **Windows (PowerShell / CMD):** `.gemini/skills/graphdb/scripts/graphdb-win.exe`

**Variable Definition:**
Define `${graphdb_bin}` as the path to the binary appropriate for the current operating system. Note: If you are running in WSL (Windows Subsystem for Linux), you are in a Linux environment and MUST use the Linux binary.

**Base Command:** `${graphdb_bin} <command> [options]`

## Setup & Infrastructure

### Installation
The skill relies on a pre-compiled Go binary (`${graphdb_bin}`).
If it does not exist you must STOP and return an ERROR to the user.

### Environment Variables
The tool automatically inherits the following environment variables. Assume they are already configured correctly via the `.env` file or host system. 
*   `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD` (Required for database connection)
*   `GOOGLE_CLOUD_PROJECT` (Required for Vertex AI embeddings)
*   `GOOGLE_CLOUD_LOCATION` (Default: `global`)
*   `GRAPHDB_DIR` (Optional: Sets the base directory for source files and state management. Equivalent to the `-dir` flag but persistent across all commands.)
*   `GEMINI_BATCH_GCS_BUCKET` (Optional: Target Google Cloud Storage bucket used for staging inputs and outputs for Vertex AI Batch jobs.)


## Operational Guidelines for Agents

### Handling Long-Running Commands (`build-all`, `enrich-features`)
When running `build-all` or `enrich-features` on exceptionally large repositories (e.g., 30k+ files), the process can take many minutes (especially the LLM embedding and clustering phases).
* **Inline Execution (Required)**: You MUST **NOT** use `run_shell_command` with `is_background: true` when invoking `build-all`. The tool is specifically designed to show progress output without flooding the context window.
* **Optimized Output**: The GraphDB CLI has "smart TTY detection". When you execute it via the `run_shell_command` tool, it detects it is running headlessly and disables aggressive progress bar updates, only emitting logs at 10% increments. This drastically reduces verbosity and prevents you from hitting output truncation limits. You can safely run these commands inline and wait for them to finish.
* **No Redirection**: Do not redirect output to log files for monitoring; the inline logs are the intended way to track progress while keeping the context lean.

### Destructive Commands
**CRITICAL RULES FOR DESTRUCTIVE COMMANDS:**
* `build-all` or any command that could be destructive to the graph database data *MUST* under all circumstances ALWAYS have explicit user approval before execution. Do not bypass this rule.

### Credentials
**CRITICAL RULES FOR CREDENTIALS:**
1. You must **NEVER** explicitly set, export, or pass environment variables (like `NEO4J_PASSWORD=...`) in your bash commands. 
2. You must rely purely on the Go binary's internal `.env` loading. 
3. If a command fails due to an `Unauthorized` or authentication error, **STOP**. Do not try to guess or brute-force the password. Report the failure directly to the user and state that the credentials in their environment or `.env` file appear to be invalid or missing.
4. You must never update the .env file without explicit approval from the user.

### Model & Location Errors
**CRITICAL RULES FOR MODEL ERRORS:**
If a command (especially `enrich-features`, `build-all`, or any semantic query) fails with a **404 error** related to the Vertex AI model or location:
1. **STOP IMMEDIATELY**. Do not attempt to retry or automatically switch models/regions.
2. **INTERVENTION REQUIRED**: This indicates that either the `GOOGLE_CLOUD_LOCATION` or the `GEMINI_EMBEDDING_MODEL` (or related model settings) in the `.env` file is mistyped or unsupported in that region.
3. Report the exact error to the user and state that they must verify and correct their `.env` configuration before you can proceed.

## Workflows

### 1. The "One-Shot" Build (Recommended)
To build the entire graph from scratch (Ingest -> Import -> All Enrichment Phases), use the `build-all` command. This is the standard entry point for all new projects.

*   **Synchronous Mode:**
    ```bash
    ${graphdb_bin} build-all
    ```
*   **Asynchronous Batch Mode:** Best for large codebases. Submits prompts to Vertex AI Batch API, staging inputs in GCS.
    1. Submit structural build and batch enrichment:
       ```bash
       ${graphdb_bin} build-all --batch --gcs-bucket <your-gcs-bucket-name>
       ```
    2. Resume and finalize remaining phases once batch completes:
       ```bash
       ${graphdb_bin} build-all --resume
       ```

### 2. Status & Incremental Ingestion
Before rebuilding, check if the graph is already in sync with your local code:
1. Get local commit: `git rev-parse HEAD`
2. Get graph commit: `${graphdb_bin} query -type status`
3. **Incremental Ingestion:** NEVER begin any ingestion without explicit approval from the user. If you have small changes, the `build-all` command automatically detects the state (using the current directory or `GRAPHDB_DIR` for baseline lookup) and only processes modified files if possible.

## Advanced: Manual Pipeline
If a specific phase fails or you need granular control, you can run the steps manually:

**Step 1: Ingest**
```bash
${graphdb_bin} ingest -nodes nodes.jsonl -edges edges.jsonl
```

**Step 2: Import**
```bash
${graphdb_bin} import -nodes nodes.jsonl -edges edges.jsonl
```

**Step 3: Feature Enrichment**
*   **Synchronous Mode (Default):** Runs inline queries synchronously against the model.
    ```bash
    ${graphdb_bin} enrich-features
    ```
*   **Asynchronous Batch Mode:** Best for large codebases. Submits prompts to Vertex AI Batch API, staging input/output in GCS.
    1. **Submit Batch Job:**
       ```bash
       ${graphdb_bin} enrich-features --batch --gcs-bucket <your-gcs-bucket-name>
       ```
    2. **Check & Import Completed Results:**
       ```bash
       ${graphdb_bin} enrich-features --check-batch
       ```


**Step 4: Contamination Analysis**
```bash
${graphdb_bin} enrich-contamination
```

**Step 5: Git History**
```bash
${graphdb_bin} enrich-history
```

**Step 6: Test Linkage**
```bash
${graphdb_bin} enrich-tests
```

### 3. Analysis & Querying
The primary way to interact with the graph is via the `query` command.

**Base Syntax:**
```bash
${graphdb_bin} query -type <type> -target "<search_term>" [options]
```

**Global Query Options:**
*   `-dir <path>`: Base directory for source files. **Critical:** Also used to retrieve/store directory-specific state (commit hashes) for incremental ingestion.
*   `-limit <int>`: Restricts the number of results returned (Default: 10). Primarily affects semantic searches (`search-features`, `search-similar`, `hybrid-context`) and dependency bounds (`neighbors`).
*   `-summary`: (Boolean) If provided, simplifies the JSON output to only include structural metrics/counts instead of full arrays of nodes. Excellent for preventing context-window bloat when querying nodes with massive fan-out (e.g., getting counts of dependencies rather than the full list).

#### Supported Languages & FQN Formats
Structural queries utilize "Fully Qualified Names" (FQN). While the internal database IDs are more complex (including labels and signatures), the query engine is polymorphic and accepts simple FQNs or exact IDs.

*   **C# / .NET / VB.NET:** `Namespace.Class.Method` (No file path)
*   **Java:** `Package.Class.Method` (No file path)
*   **C / C++:** `FilePath:Namespace::Class::Method` (or `FilePath:Class::Method`)
*   **TypeScript:** `FilePath:Class.Method` or `FilePath:Function` (e.g., `src/app.ts:MyClass.myMethod`)
*   **SQL:** `FilePath:Schema.ObjectName` or `FilePath:ObjectName`

#### Query Types Reference

| Type | Description | Target | Options |
| :--- | :--- | :--- | :--- |
| `search-features` | **Intent Search.** Find features/concepts using vector search. | Natural language query | `-limit` |
| `search-similar` | **Code Search.** Find functions semantically similar to a query. | Natural language or code snippet | `-limit` |
| `duplicates` | **Code Search.** Find pairs of duplicated functions using vector embeddings globally. | (Ignored) | `-limit`, `-similarity` (default 0.5) |
| `neighbors` / `test-context` | **Dependency Analysis.** Find immediate callers and callees. | Function Name (exact) | `-depth` |
| `coverage` | **Test Analysis.** Returns tests that cover a specific production function or method. | Function Name/ID | |
| `hybrid-context` | **Combined.** Structural neighbors + semantic similarities. Great for refactoring. | Function Name | `-depth`, `-limit` |
| `impact` | **Risk Analysis.** What other parts of the system behave differently if I change this? | Function Name | `-depth` |
| `what-if` | **Impact Simulation.** Computes the impact of hypothetical node removals (Severed Edges, Orphaned Nodes, etc.) for Strangler Fig planning. | Function Name/ID | `-target2 <Target 2>` |
| `hotspots` | **Risk Analysis.** Find functions with high complexity that change frequently. | (Ignored) | `-module <regex>` |
| `globals` | **State Analysis.** Find global variables used by a function. | Function Name | |
| `seams` | **Architecture.** Identify Pinch Points (chokepoints with high internal fan-in and high volatile fan-out). | (Ignored) | `-module <regex>` |
| `semantic-seams` | **Architecture.** Identify SRP violations and conceptual seams within a single file/class using vector embeddings. | (Ignored) | `-similarity <float>` |
| `locate-usage` | **Trace.** Find path/usage between two functions. | Function 1 | `-target2 <Function 2>` |
| `fetch-source` | **Read.** Fetch the full implementation (signature + body) of a function. | Function Name / FQN | Recommended: Use `fqn` to resolve overloads or duplicate names. |
| `explore-domain` | **Discovery.** Explore the domain model around a concept. | Concept/Entity Name | |
| `traverse` | **Raw Traversal.** Explore graph relationships directly. | Node ID / Name | `-edge-types`, `-direction`, `-depth` |
| `cypher` | **Advanced.** Run a raw, read-only Cypher query directly against the graph database. | (Ignored) | `-cypher "<query>"` |
| `status` | **Verification.** Check the git commit hash stored in the graph. | (None) | `-dir` |

### 4. Web Visualizer (HTTP Server)
The GraphDB skill includes a web-based visualizer for exploring the graph, viewing clusters, and performing interactive queries.

**Command:**
```bash
${graphdb_bin} serve -port 8080
```
*   *Options:*
    *   `-port`: The port to run the server on (default: `8080`).
    *   `-location`: GCP Location for semantic searches (default: `global`).
    *   `-model`: Embedding model to use for semantic searches (default: loaded from `.env`).

**Operational Note:**
When running the server in a remote environment (e.g., a Cloud VM), you may need to use SSH port forwarding to access the UI locally:
`ssh -L 8080:localhost:8080 your-vm-address`

## Operational Guidelines
*   **Output Parsing:** The tool returns JSON. Parse it and present a concise summary (bullet points, mermaid diagrams, or tables).
*   **Exact Names:** Structural queries (`neighbors`, `impact`, `coverage`, `what-if`) are **polymorphic**. You can provide the Node `ID` (e.g., `Function:Namespace.Class.Method:()`), the `fqn` (e.g., `Namespace.Class.Method`), or the simple `name` (e.g., `Method`).
    *   **Recommendation:** Use the `fqn` for the most robust results across overloads and distinct modules.
    *   **Test Analysis:** Always prefer `coverage` over `neighbors` when specifically looking for tests, as it leverages explicit links from `enrich-tests`.
    *   **Impact Analysis:** Use `impact` for general risk assessment and `what-if` for simulation-based planning (e.g., Strangler Fig pattern).
    *   **CRITICAL RULE:** If you only have a partial name or an ambiguous symbol from the user, **DO NOT use `grep` or text search** to find the fully qualified name. Instead, you MUST use `search-similar` (semantic search) first to locate the exact node `ID` or `fqn` before running structural queries.
*   **Context:** Always mention the source file and line number when discussing a function.
*   **Missing Data:** If a query returns empty, verify the spelling of the function/module name or try a semantic search.
