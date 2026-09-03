---
type: Pipeline Phase
title: "Phase 2: Graph Loading & Index Persistence"
description: High-throughput ingestion of JSONL streams into Neo4j 5.x using transactional UNWIND batches, schema constraints, and vector indexes.
tags: [pipeline, phase2, import, neo4j, cypher, unwind, indexes]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: neo4j-loader
    resource: /internal/loader/neo4j_loader.go
    title: Neo4j Bulk Batch Loader
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: cmd-import
    resource: /cmd/graphdb/cmd_import.go
    title: CLI Import Command Handler
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Phase 2: Graph Loading & Index Persistence

## 1. Overview & Operational Contract

* **CLI Command:** `graphdb import -nodes <nodes.jsonl> -edges <edges.jsonl> [flags]`[^cmd-import]
* **Inputs:** Streaming JSONL files produced by Phase 1 Ingestion.
* **Outputs:** Fully indexed, persistent Code Property Graph in Neo4j 5.x.
* **Dependencies:** Running Neo4j Community Edition 5.x instance accessible via Bolt protocol.

Phase 2 manages the transactional loading of nodes, relationships, uniqueness constraints, and vector search indexes into Neo4j.[^neo4j-loader]

---

## 2. Ingestion Pipeline Architecture

```mermaid
flowchart TD
    A[("Streaming JSONL\nnodes.jsonl / edges.jsonl")] --> B["Stream Scanner (Buffer Reader)"]
    B --> C["Batch Accumulator\n(Default: 1,000 items / batch)"]

    subgraph Neo4j_Setup ["1. Schema & Index Initialization"]
        S1["Ensure Uniqueness Constraints\n(File, Class, Function, Global, Feature)"]
        S2["Ensure Vector Indexes\n(Function.embedding, Feature.embedding 768d Cosine)"]
    end

    subgraph Bulk_Execution ["2. Transactional Cypher UNWIND"]
        T1["Load Nodes: UNWIND $batch AS row MERGE ..."]
        T2["Load Edges: UNWIND $batch AS row MATCH (src), (tgt) CREATE ..."]
    end

    Neo4j_Setup --> Bulk_Execution
    C --> Bulk_Execution
    Bulk_Execution --> DB[("Neo4j 5.x Database")]

    classDef proc fill:#e3f2fd,stroke:#1565c0,stroke-width:1px;
    classDef store fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px;
    class B,C,S1,S2,T1,T2 proc;
    class A,DB store;
```

---

## 3. Implementation Details

### 3.1 Constraint and Index Initialization (`internal/loader/neo4j_loader.go`)
Before streaming records, the loader asserts required database constraints:
1. **Uniqueness Constraints:** Guarantees primary key uniqueness on `id` across all labels:
   ```cypher
   CREATE CONSTRAINT IF NOT EXISTS FOR (f:Function) REQUIRE f.id IS UNIQUE;
   CREATE CONSTRAINT IF NOT EXISTS FOR (c:Class) REQUIRE c.id IS UNIQUE;
   CREATE CONSTRAINT IF NOT EXISTS FOR (file:File) REQUIRE file.id IS UNIQUE;
   CREATE CONSTRAINT IF NOT EXISTS FOR (feat:Feature) REQUIRE feat.id IS UNIQUE;
   CREATE CONSTRAINT IF NOT EXISTS FOR (comm:StructuralCommunity) REQUIRE comm.id IS UNIQUE;
   ```
2. **Vector Index Creation:** Creates high-performance vector search indexes for 768-dimensional embeddings:
   ```cypher
   CREATE VECTOR INDEX function_embedding_idx IF NOT EXISTS
   FOR (f:Function) ON (f.embedding)
   OPTIONS {indexConfig: {`vector.dimensions`: 768, `vector.similarity_function`: 'cosine'}};
   ```

### 3.2 High-Throughput Cypher `UNWIND` Pattern
Individual `CREATE` or `MERGE` queries incur immense network and transaction overhead. The loader buffers records into batches of 1,000 items and executes parameterized Cypher:
```cypher
UNWIND $batch AS item
MERGE (f:Function {id: item.id})
SET f.name = item.name,
    f.file = item.file,
    f.start_line = item.start_line,
    f.end_line = item.end_line,
    f.is_test = item.is_test
```
* **Why UNWIND?** It amortizes transaction commit overhead, achieving insertion speeds of 15,000–30,000 nodes/second on commodity hardware.

### 3.3 Clean-Slate Wipes (`--clean`)
When re-indexing a repository from scratch, old nodes and relationships can linger. Passing the `--clean` flag triggers `RecreateDatabase()`, which safely purges all data, constraints, and indexes before starting the import.

---

## 4. CLI Flags Reference

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-nodes` | `graph_data/nodes.jsonl` | Path to JSONL file containing node definitions. |
| `-edges` | `graph_data/edges.jsonl` | Path to JSONL file containing relationship definitions. |
| `-batch-size` | `1000` | Number of records committed per Cypher transaction. |
| `-clean` | `false` | Wipe all existing nodes, relationships, and indexes before importing. |

[^cmd-import]: [`cmd_import.go`](file:///home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_import.go)
[^neo4j-loader]: [`neo4j_loader.go`](file:///home/jasondel/dev/graphdb-skill/internal/loader/neo4j_loader.go)
