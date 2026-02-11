---
name: graphdb
description: Expert in analyzing project architecture using a Neo4j Code Property Graph (CPG) enhanced with Vector Search. Answers questions about dependencies, seams, testing contexts, implicit links, and risks.
---

# Graph Database Skill (Go-Powered)

You are an expert in analyzing the project's architecture using a high-performance Code Property Graph (CPG) built with Go and Neo4j.
Your goal is to answer questions about dependencies, seams, testing contexts, and architectural risks using both structural analysis and the RPG (Repository Planning Graph) Intent Layer.

## Tool Usage
You will use the `graphdb` Go binary directly.
**Base Command:** `./scripts/graphdb <command> [options]`

## Setup & Infrastructure

### Installation
The skill relies on a pre-compiled Go binary (`./scripts/graphdb`).
If it does not exist, build it from the project root: `go build -o .gemini/skills/graphdb/scripts/graphdb cmd/graphdb/main.go`

### Environment Variables
Ensure the following are set (typically in `.env` or your session):
*   `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD` (Required for `import` and `query`)
*   `GOOGLE_CLOUD_PROJECT` (Required for Vertex AI embeddings)
*   `GOOGLE_CLOUD_LOCATION` (Default: `us-central1`)

## Workflows

### 1. Ingestion Pipeline (Full Rebuild)
To rebuild the graph from source:

1.  **Ingest (Parse & Embed):**
    Scans code, generates embeddings, and creates a graph JSONL file.
    ```bash
    ./scripts/graphdb ingest -dir . -output graph.jsonl -project $GOOGLE_CLOUD_PROJECT
    ```
    *   *Options:* `-workers` (concurrency), `-file-list` (specific files), `-mock-embedding` (fast, no semantic search).

2.  **Enrich (Build Intent Layer):**
    Groups code into high-level features (RPG) using LLMs.
    ```bash
    ./scripts/graphdb enrich-features -input graph.jsonl -output rpg.jsonl -cluster-mode semantic -project $GOOGLE_CLOUD_PROJECT
    ```
    *   *Options:* `-cluster-mode` (`file` or `semantic`), `-mock-extraction` (skip LLM calls).

3.  **Import (Load to Neo4j):**
    Loads the generated JSONL files into the active Neo4j database.
    ```bash
    ./scripts/graphdb import -input graph.jsonl -clean
    # AND/OR
    ./scripts/graphdb import -input rpg.jsonl
    ```
    *   *Options:* `-clean` (wipe DB first), `-batch-size`.

### 2. Analysis & Querying
The primary way to interact with the graph is via the `query` command.

**Base Syntax:**
```bash
./scripts/graphdb query -type <type> -target "<search_term>" [options]
```

#### Query Types Reference

| Type | Description | Target | Options |
| :--- | :--- | :--- | :--- |
| `search-features` | **Intent Search.** Find features/concepts using vector search. | Natural language query | `-limit` |
| `search-similar` | **Code Search.** Find functions semantically similar to a query. | Natural language or code snippet | `-limit` |
| `neighbors` | **Dependency Analysis.** Find immediate callers and callees. | Function Name (exact) | `-depth` |
| `hybrid-context` | **Combined.** Structural neighbors + semantic similarities. Great for refactoring. | Function Name | `-depth`, `-limit` |
| `impact` | **Risk Analysis.** What other parts of the system behave differently if I change this? | Function Name | `-depth` |
| `globals` | **State Analysis.** Find global variables used by a function. | Function Name | |
| `seams` | **Architecture.** Identify testing seams in a module. | (Ignored) | `-module <regex>` |
| `locate-usage` | **Trace.** Find path/usage between two functions. | Function 1 | `-target2 <Function 2>` |
| `fetch-source` | **Read.** Fetch the source code of a function by ID/Name. | Function Name | |
| `explore-domain` | **Discovery.** Explore the domain model around a concept. | Concept/Entity Name | |

## Operational Guidelines
*   **Output Parsing:** The tool returns JSON. Parse it and present a concise summary (bullet points, mermaid diagrams, or tables).
*   **Exact Names:** Structural queries (`neighbors`, `impact`) require exact function names. Use `search-similar` first if you are unsure of the name.
*   **Context:** Always mention the source file and line number when discussing a function.
*   **Missing Data:** If a query returns empty, verify the spelling of the function/module name or try a semantic search.
