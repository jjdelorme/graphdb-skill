---
type: Pipeline Phase
title: "Phase 1: Deterministic Structural Ingestion"
description: Architecture and operation of the offline Tree-sitter parsing engine, file walker, concurrent worker pool, and JSONL streaming serializer.
tags: [pipeline, phase1, ingestion, tree-sitter, ast, jsonl]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: walker-impl
    resource: /internal/ingest/walker.go
    title: Directory Walker & Gitignore Processor
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: cmd-ingest
    resource: /cmd/graphdb/cmd_ingest.go
    title: CLI Ingest Command Handler
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: jsonl-emitter
    resource: /internal/storage/emitter.go
    title: Streaming JSONL Node & Edge Emitter
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Phase 1: Deterministic Structural Ingestion

## 1. Overview & Operational Contract

* **CLI Command:** `graphdb ingest -dir <source-path> [flags]`[^cmd-ingest]
* **Inputs:** Polyglot source code repository on disk.
* **Outputs:** Streaming JSONL files containing physical graph entities (`nodes.jsonl`, `edges.jsonl`).
* **Dependencies:** None. Completely offline — requires zero network access, zero cloud credentials, and no database connection.

Phase 1 converts raw source code into a normalized, language-agnostic Code Property Graph representation.[^walker-impl]

---

## 2. Ingestion Architecture & Execution Flow

```mermaid
flowchart TD
    A["File System Root"] --> B["Walker (Respects .gitignore & sub-repos)"]
    B -->|File Path Stream| C["Buffered Channel (Path Queue)"]

    subgraph Pool ["Concurrent Worker Pool (-workers N)"]
        W1["Worker 1: Extension Matcher"]
        W2["Worker 2: Extension Matcher"]
        WN["Worker N: Extension Matcher"]
    end

    C --> W1
    C --> W2
    C --> WN

    W1 -->|CGO Tree-sitter| P1["Language AST Parsers\n(C++, C#, Java, TS, Py, SQL)"]
    W2 -->|CGO Tree-sitter| P1
    WN -->|CGO Tree-sitter| P1

    P1 --> E["Entity & Edge Extractor\n(Classes, Functions, Calls, Globals)"]
    E -->|Thread-safe JSONL Write| Out[("Streaming JSONL Output\nnodes.jsonl / edges.jsonl")]

    classDef proc fill:#e3f2fd,stroke:#1565c0,stroke-width:1px;
    classDef store fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px;
    class B,C,W1,W2,WN,P1,E proc;
    class Out store;
```

---

## 3. Key Components & Implementation Details

### 3.1 Filesystem Walker (`internal/ingest/walker.go`)
* **Gitignore Compliance:** Dynamically loads and respects `.gitignore` rules at every directory depth. Build directories (`node_modules`, `bin`, `obj`, `target`, `dist`, `.venv`) are pruned before traversing.
* **Sub-repository Isolation:** Detects nested `.git` directories and Git worktrees to prevent traversing unrelated checkouts.
* **Worker Dispatch:** Dispatches discovered files across a configurable number of worker goroutines (defaults to runtime CPU core count via `-workers`).

### 3.2 Tree-sitter CGO Parsing
* Each worker selects the appropriate Tree-sitter grammar based on file extension.
* Generates a Concrete Syntax Tree (CST) in memory without executing build tools or compiler passes.
* Extracts code symbols:
  * Container types: `File`, `Class`, `Interface`, `Table`.
  * Executable blocks: `Function`, `Constructor`.
  * State variables: `Field`, `Global`.
  * Relational edges: `CALLS`, `USES`, `USES_GLOBAL`, `HAS_METHOD`, `DEFINES`, `EXTENDS`, `IMPLEMENTS`, `DEPENDS_ON`, `DEFINED_IN`.

### 3.3 Test Code Tagging
During AST traversal, test files and test functions are identified via structural conventions (e.g., `Test*`, `*Test`, `*_spec.js`, `@Test` annotations). These nodes are marked with `is_test: true`. This metadata ensures test code does not skew business domain clustering while enabling Phase 5 test-linkage analysis.

### 3.4 Streaming JSONL Serialization (`internal/storage/emitter.go`)
* Emits newline-delimited JSON (`nodes.jsonl` and `edges.jsonl`).[^jsonl-emitter]
* **Why decouple Ingestion from Import?**
  1. **CPU vs. I/O Separation:** Parsing is CPU-bound (Tree-sitter CGO), whereas loading into Neo4j is disk and I/O-bound. Decoupling allows independent performance optimization.
  2. **Auditability & Diffing:** Emitted JSONL can be inspected, piped to other tools, versioned, or reloaded into Neo4j without re-running AST parsing.
  3. **Zero Embedding Premature Optimization:** Bare function names lack semantic context. By omitting vector embeddings here, the pipeline ensures embeddings are created only after LLM atomic feature extraction in Phase 3.

---

## 4. CLI Flags Reference

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-dir` | `.` | Root directory of the source code repository to ingest. |
| `-output` | `./graph_data` | Target directory for generated `nodes.jsonl` and `edges.jsonl`. |
| `-workers` | `runtime.NumCPU()` | Number of concurrent parsing workers. |
| `-since-commit`| `""` | Incremental mode: only parse files modified since the specified Git commit SHA. |

[^walker-impl]: [`walker.go`](file:///home/jasondel/dev/graphdb-skill/internal/ingest/walker.go)
[^cmd-ingest]: [`cmd_ingest.go`](file:///home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_ingest.go)
[^jsonl-emitter]: [`emitter.go`](file:///home/jasondel/dev/graphdb-skill/internal/storage/emitter.go)
