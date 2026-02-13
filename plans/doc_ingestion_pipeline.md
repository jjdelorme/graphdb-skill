# Documentation Update Plan: Ingestion Pipeline Overview

## 📋 Todo Checklist
- [ ] Update `GRAPHDB_OVERVIEW.md` with the new section.
- [ ] Verify the content aligns with the actual implementation in `internal/ingest`, `internal/rpg`, and `internal/loader`.

## 🔍 Analysis & Investigation
The `GRAPHDB_OVERVIEW.md` file currently covers the Graph Schema and Visual Representation. It lacks a detailed explanation of *how* the graph is built. The user has requested a specific 4-phase breakdown of the Ingestion Pipeline.

**Code Verification:**
1.  **Phase 1 (Extraction):** Confirmed in `internal/ingest/worker.go` (Worker Pool) and `internal/analysis` (Tree-sitter parsers, definition queries).
2.  **Phase 2 (Embedding):** Confirmed in `internal/embedding/vertex.go` (Vertex AI client).
3.  **Phase 3 (RPG Construction):** Confirmed in `internal/rpg/builder.go` (Hierarchy), `internal/rpg/cluster_semantic.go` (Clustering), and `internal/rpg/extractor.go` (Atomic Extraction).
4.  **Phase 4 (Loading):** Confirmed in `internal/loader/neo4j_loader.go` (UNWIND batching).

## 📝 Implementation Plan

### Step 1: Update `GRAPHDB_OVERVIEW.md`
*   **Action:** Append the following section to the end of `GRAPHDB_OVERVIEW.md`.

```markdown
---

## 4. The Ingestion Pipeline

The transition from raw code to a queriable knowledge graph occurs in four distinct phases. This pipeline is designed to be deterministic where possible (parsing) and probabilistic where necessary (intent understanding).

### Phase 1: Extraction (Parsing)
*   **Mechanism:** A high-performance Go binary (`graphdb extract`) uses a worker pool to walk the file system in parallel.
*   **Parsing:** Uses **Tree-sitter** bindings to generate a Concrete Syntax Tree (CST) for every file.
*   **Graph Construction:**
    *   **Nodes:** Extracts structural entities: `File`, `Class`, `Function`, `Field`, `Global`.
    *   **Edges:** Identifies relationships: `HAS_METHOD`, `DEFINES`, `INHERITS`, `CALLS`, `USES`.
*   **Resolution:** Performs **Systemic Resolution** on the fly, resolving imports (e.g., `using System;`, `import java.util.*;`) to link types across file boundaries.

### Phase 2: Embedding (Vectorization)
*   **Mechanism:** The pipeline isolates function bodies and signatures.
*   **Model:** Sends text to Google's Vertex AI (configured via `GEMINI_EMBEDDING_MODEL`, e.g., `text-embedding-004` or `gemini-embedding-001`).
*   **Storage:** The resulting 768-dimensional vectors are stored as properties on `Function` nodes, enabling semantic search and clustering.

### Phase 3: RPG Construction (Intent Generation)
This phase builds the "Intent Layer" (RPG) on top of the physical code.

1.  **Atomic Feature Extraction:** An LLM analyzes each function to extract a "Verb-Object" descriptor (e.g., "validates email", "hashes password").
2.  **Semantic Clustering:** The **EmbeddingClusterer** uses K-Means++ on the function vectors to group code with similar semantic meaning, regardless of directory structure.
3.  **Summarization:** An LLM generates a concise name and description for each cluster (e.g., "User Authentication").
4.  **Linking:** Creates `IMPLEMENTS` edges connecting the physical `Function` nodes to the new logical `Feature` nodes.

### Phase 4: Loading (Persistence)
*   **Format:** The pipeline emits a stream of JSONL records (Nodes and Edges).
*   **Ingestion:** The `graphdb import` command reads the JSONL stream.
*   **Batching:** Uses Cypher `UNWIND` clauses to batch-insert thousands of records per transaction into Neo4j, ensuring high throughput and transactional integrity.
```

## 🎯 Success Criteria
*   The `GRAPHDB_OVERVIEW.md` file contains the new section "4. The Ingestion Pipeline".
*   The description accurately reflects the codebase structure and behavior.
