# Graph Database Skill & Infrastructure

**Status:** Active
**Version:** 1.1 (Hybrid Graph)
**Location:** `.gemini/skills/graphdb/`

## 📖 Overview

The **GraphDB Skill** is a specialized subsystem designed to support the analysis and modernization of large codebases. It ingests C++, C#, VB.NET, ASP.NET, and SQL source code into a **Neo4j knowledge graph**, enabling surgical dependency analysis, seam identification, and architectural insights.

It employs a **Hybrid Architecture** combining:
1.  **Code Property Graph (CPG)**: Precise structural analysis (Calls, Inheritance, Global Variable usage).
2.  **Vector Embeddings**: Semantic search for finding implicit dependencies (e.g., cross-language calls, string-based APIs).

---

## 🏗️ Architecture

```
┌──────────────────────┐      ┌──────────────────────┐      ┌──────────────────────┐
│  Source Code         │  ->  │   Ingestion Pipeline  │  ->  │    Neo4j Database    │
│  (C++, C#, VB, etc)  │      │  (Node.js Scripts)   │      │   (Hybrid Graph)     │
└──────────────────────┘      └──────────────────────┘      └──────────────────────┘
                                                                       ^
                                                                       |
┌──────────────────────┐      ┌──────────────────────┐      ┌──────────────────────┐
│  Gemini CLI Agent    │  <-  │    Skill Interface   │  <-  │   Query Tools (CLI)  │
│  (Refactoring Logic) │      │   (SKILL.md Prompts) │      │  (query_graph.js)    │
└──────────────────────┘      └──────────────────────┘      └──────────────────────┘
```

### Key Entities (Schema)
*   **`Function`**: Code blocks with metrics (complexity, LOC) and **Vector Embeddings**.
*   **`File`**: Source files with Git history metrics (change frequency).
*   **`Global`**: Global variables/state.
*   **`Class`**: Classes/structs (including inheritance).
*   **Edges**: `CALLS`, `DEFINED_IN`, `USES_GLOBAL`, `MEMBER_OF`, `INHERITS_FROM`.

---

## 🚀 Setup & Installation

### Prerequisites
*   **Node.js**: v20+
*   **Neo4j Community Edition**: v5.x (Local) with Vector Index support (v5.11+).
*   **Git**: Available in PATH

### 1. Database Configuration
The connection is managed via the `.env` file in the project root.
*   **Host**: `localhost`
*   **Protocol**: `bolt://localhost:7687`
*   **Credentials**: Defined in `.env` (`NEO4J_USER`, `NEO4J_PASSWORD`)

### 2. Google Cloud Configuration (for Vector Search)
Required environment variables in `.env`:
*   `GOOGLE_CLOUD_PROJECT`: GCP Project ID.
*   `GOOGLE_CLOUD_LOCATION`: GCP Region (e.g., `us-central1`).
*   `GEMINI_EMBEDDING_MODEL`: `models/gemini-embedding-001` (default).

### 3. Tool Installation
```bash
cd .gemini/skills/graphdb/
npm install
```

---

## 🔄 Ingestion Pipeline

To build the graph from source, run the following steps in order from the root directory.

### Step 1: Extract Graph Data
Parses source code using Tree-sitter.
```bash
node .gemini/skills/graphdb/extraction/extract_graph.js
```
*Output: `.gemini/graph_data/nodes.json`, `.gemini/graph_data/edges.json`*

### Step 2: Import to Neo4j
Loads data into the database.
```bash
node .gemini/skills/graphdb/scripts/import_to_neo4j.js
```

### Step 3: Enrich (Git & Contamination)
Adds history metrics and UI contamination flags (defaults to MFC patterns).
```bash
node .gemini/skills/graphdb/scripts/analyze_git_history.js
node .gemini/skills/graphdb/scripts/propagate_contamination.js
```

### Step 4: Enrich (Vector Embeddings)
Generates semantic embeddings for functions to enable semantic search.
*Requires GCP Credentials.*
```bash
node .gemini/skills/graphdb/scripts/enrich_vectors.js
```

---

## 🔍 Querying the Graph

The agent uses the CLI tool `query_graph.js`. Humans can also use it for exploration.

**Base Command:**
```bash
node .gemini/skills/graphdb/scripts/query_graph.js <command> [options]
```

### Common Commands

| Command | Description | Example |
| :--- | :--- | :--- |
| **`ui-contamination`** | Stats on UI-coupled vs. pure logic | `... ui-contamination --module <name>` |
| **`seams`** | Find entry points for refactoring | `... seams --module <name>` |
| **`test-context`** | List dependencies for test harnesses | `... test-context --function <name>` |
| **`hotspots`** | Find high-risk (complex + frequent changes) code | `... hotspots --module <name>` |
| **`globals`** | Show global variable usage | `... globals --module <name>` |

### Semantic Search
Find implicit links using vector search.
```bash
node .gemini/skills/graphdb/scripts/find_implicit_links.js --query "natural language query"
```

> **📘 Detailed Examples:** See **[QUERY_EXAMPLES.md](QUERY_EXAMPLES.md)** for comprehensive usage scenarios and output descriptions.

---

## 📂 Maintenance & Development

*   **Skill Definition**: `.gemini/skills/graphdb/SKILL.md` (Agent instructions).
*   **Scripts**: `.gemini/skills/graphdb/scripts/` (Import/Query logic).
*   **Parsers**: `.gemini/skills/graphdb/extraction/` (Tree-sitter logic).
