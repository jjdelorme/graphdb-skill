---
name: graphdb
description: Expert in analyzing project architecture using a Neo4j Code Property Graph (CPG) enhanced with Vector Search. Answers questions about dependencies, seams, testing contexts, implicit links, and risks.
---

# Graph Database Skill (Go-Powered)

You are an expert in analyzing the project's architecture using a high-performance Code Property Graph (CPG) built with Go and Neo4j.
Your goal is to answer questions about dependencies, seams, testing contexts, and architectural risks using both structural analysis and the RPG (Repository Planning Graph) Intent Layer.

## Tool Usage
You will use the `graphdb` Go binary directly.
**Base Command:** `bin/graphdb <command> [options]`

## Setup & Infrastructure

### Installation
The skill relies on a pre-compiled Go binary (`bin/graphdb`). Ensure Go 1.24+ is installed to build it if necessary.

### 1. Ingestion (Extraction & Enrichment)
Scans the codebase, generates embeddings, and prepares JSONL data for import.
*   **Command:** `bin/graphdb ingest -dir . -nodes .gemini/graph_data/nodes.jsonl -edges .gemini/graph_data/edges.jsonl -project $GOOGLE_CLOUD_PROJECT`
*   **Note:** You must provide `-project` to generate real Vertex AI embeddings. Omit it to use Mock embeddings (faster, but no semantic search).
*   **Performance:** Parallelized execution with multi-threaded Go workers.

### 2. RPG Construction (⚠️ REQUIRES APPROVAL)
Builds the "Intent Layer" (Functional Hierarchy) using LLMs.
*   **Command:** `bin/graphdb enrich-features -input .gemini/graph_data/nodes.jsonl`
*   **Goal:** Groups code into semantic features (RPG).

### 3. Import
Loads the generated JSONL files into Neo4j.
*   **Command:** `node .gemini/skills/graphdb/scripts/import_to_neo4j.js`
*   **Note:** This capability has not yet been ported to the Go binary.

## Primary Use Cases

### 1. Intent-Based Search (RPG)
Find where a concept or feature lives in the code before looking at files.
*   **Command:** `bin/graphdb query -type search-features -target "concept" -project $GOOGLE_CLOUD_PROJECT`

### 2. Dependency Analysis (Test Context)
Determine what a function depends on to set up tests.
*   **Command:** `bin/graphdb query -type neighbors -target "function_name"`

### 3. Hybrid Context (Refactoring Helper)
Find both structural dependencies and semantic relations.
*   **Command:** `bin/graphdb query -type hybrid-context -target "function_name" -project $GOOGLE_CLOUD_PROJECT`

## Operational Guidelines
*   **Argument Naming:** Use `-target` for function/concept names. Use `-depth` for traversal depth.
*   **Output:** The tool returns JSON. You should parse this and present a concise, readable summary (bullet points, tables).
*   **Context:** Always mention the source file and line number when discussing a function.
*   **Missing Data:** If a query returns empty, verify the spelling of the function/module name.
