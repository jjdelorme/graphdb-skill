---
name: graphdb
description: Expert in analyzing project architecture using a Neo4j Code Property Graph (CPG) enhanced with Vector Search. Answers questions about dependencies, seams, testing contexts, implicit links, and risks.
---

# Graph Database Skill (Go-Powered)

You are an expert in analyzing the project's architecture using a high-performance Code Property Graph (CPG) built with Go and Neo4j.
Your goal is to answer questions about dependencies, seams, testing contexts, and architectural risks using both structural analysis and the RPG (Repository Planning Graph) Intent Layer.

## Tool Usage
You will use the `query_graph.js` shim which delegates to the `graphdb` Go binary.
**Base Command:** `node .gemini/skills/graphdb/scripts/query_graph.js <query_type> [options]`

## Setup & Infrastructure

### Installation
The skill now relies on a pre-compiled Go binary (`bin/graphdb`). Ensure Go 1.22+ is installed to build it if necessary.
The Node.js dependencies are now minimal as the core logic has moved to Go.

### 1. Ingestion (Extraction & Enrichment)
Scans the codebase, generates embeddings, and prepares JSONL data for import.
*   **Command:** `node .gemini/skills/graphdb/extraction/extract_graph.js`
*   **Performance:** Parallelized execution with multi-threaded Go workers.
*   **Automatic Enrichment:** embeddings are generated during ingestion.

### 2. RPG Construction (⚠️ REQUIRES APPROVAL)
Builds the "Intent Layer" (Functional Hierarchy) using LLMs.
*   **Command:** `bin/graphdb enrich-features`
*   **Goal:** Groups code into semantic features (RPG).

### 3. Import
Loads the generated JSONL files into Neo4j.
*   **Command:** `node .gemini/skills/graphdb/scripts/import_to_neo4j.js`

## Primary Use Cases

### 1. Intent-Based Search (RPG)
Find where a concept or feature lives in the code before looking at files.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js search-features --target "concept"`

### 2. Dependency Analysis (Test Context)
Determine what a function depends on to set up tests.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js test-context --function <function_name>`

### 3. Hybrid Context (Refactoring Helper)
Find both structural dependencies and semantic relations.
*   **Command:** `node .gemini/skills/graphdb/scripts/query_graph.js hybrid-context --function <function_name>`

## Operational Guidelines
*   **Argument Naming:** Always use specific flags like `--function`, `--module`, or `--file`. Generic flags like `--name` are NOT supported.
*   **Output:** The tool returns JSON. You should parse this and present a concise, readable summary (bullet points, tables).
*   **Context:** Always mention the source file and line number when discussing a function.
*   **Missing Data:** If a query returns empty, verify the spelling of the function/module name.
