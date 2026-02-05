# GraphDB Skill Ecosystem

## 📖 Project Overview

This workspace hosts the **GraphDB Skill**, a powerful subsystem for the Gemini CLI designed to analyze, visualize, and assist in the modernization of large legacy codebases.

It employs a **Hybrid Architecture** that combines:
1.  **Code Property Graph (CPG):** A Neo4j database representing precise structural relationships (Calls, Inheritance, Variable Usage) extracted from source code (C++, C#, VB.NET, SQL, etc.).
2.  **Vector Embeddings:** Semantic understanding of code functions to identify implicit links and "conceptual" dependencies that static analysis misses.

## 📂 Repository Structure

*   **`.gemini/skills/graphdb/`**: The core skill. Contains logic for parsing code (Tree-sitter), building the graph, and querying it.
*   **`.gemini/skills/neo4j-manager/`**: A utility skill for managing Neo4j Community Edition databases (handling the single-active-database limitation).
*   **`plans/`**: Strategic documentation and architectural plans.

## 🚀 Getting Started

### Prerequisites

*   **Node.js**: v20+
*   **Neo4j Community Edition**: v5.x (Local) with Vector Index support (v5.11+).
*   **Google Cloud Project**: For Vertex AI embeddings (required for Vector Search).

### Configuration (`.env`)
Ensure a `.env` file exists in the project root with the following:

```ini
# Neo4j Configuration
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=your_password

# Google Cloud (For Embeddings)
GOOGLE_CLOUD_PROJECT=your_project_id
GOOGLE_CLOUD_LOCATION=us-central1
GEMINI_EMBEDDING_MODEL=gemini-embedding-001
GEMINI_EMBEDDING_DIMENSIONS=768
```

### Installation

Install dependencies for both skills:

```bash
cd .gemini/skills/graphdb && npm install
cd ../neo4j-manager && npm install
cd ../../../ # Return to root
```

## 👨‍💻 Development Guidelines

*   **TDD is MANDATORY:** All changes must follow Red-Green-Refactor. See `test/` directories in each skill for examples.
