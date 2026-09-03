---
type: Architecture Layer
title: Physical Code Property Graph (CPG) Specification
description: Definitive schema, node types, edge semantics, and Tree-sitter AST extraction pipeline for the physical code property graph.
tags: [architecture, cpg, schema, ast, tree-sitter, neo4j]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: ingest-walker
    resource: /internal/ingest/walker.go
    title: File Walker & Worker Pool Ingestor
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: analysis-base
    resource: /internal/analysis/types.go
    title: Graph Entity & Node Definitions
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: neo4j-loader
    resource: /internal/loader/neo4j_loader.go
    title: Neo4j Schema Constraints and UNWIND Ingestor
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Physical Code Property Graph (CPG) Specification

## 1. Overview & Purpose

The **Physical Code Property Graph (CPG)** models the syntactic and structural relationships directly present in raw source code.[^ingest-walker] In the GraphDB Skill, this layer is generated deterministically, quickly, and completely offline using CGO bindings to **Tree-sitter** grammar parsers. It commits physical code entities into **Neo4j 5.x** using transactional batch loaders.[^neo4j-loader]

The CPG serves as the foundational substrate for all downstream vector embeddings, community detection algorithms, and architectural modernization queries.

---

## 2. Graph Schema Contract

Neo4j is schemaless by design, but the GraphDB Skill strictly enforces a relational contract between the Ingestion Pipeline (Producers in [`internal/analysis/`](file:///home/jasondel/dev/graphdb-skill/internal/analysis)) and the Query Engine (Consumers in [`internal/query/`](file:///home/jasondel/dev/graphdb-skill/internal/query)).[^analysis-base]

### Visual Schema Diagram

```mermaid
graph TD
    %% --- Physical Nodes ---
    subgraph Physical_Nodes ["Physical Code Entities"]
        F["File"]
        Cl["Class / Interface"]
        Fn["Function / Constructor"]
        Fd["Field / Property"]
        Gl["Global Variable"]
        Tb["SQL Table"]
    end

    %% --- Structural Ownership Edges ---
    Cl -- "HAS_METHOD" --> Fn
    Cl -- "DEFINES" --> Fd
    Cl -- "EXTENDS / IMPLEMENTS" --> Cl
    Cl -- "DEPENDS_ON" --> Cl

    %% --- Behavioral Invocation & State Edges ---
    Fn -- "CALLS" --> Fn
    Fn -- "USES" --> Fd
    Fn -- "USES_GLOBAL" --> Gl
    Fn -- "READS / WRITES" --> Tb

    %% --- Containment Edges ---
    Fn -. "DEFINED_IN" .-> F
    Cl -. "DEFINED_IN" .-> F
    Fd -. "DEFINED_IN" .-> F
    Gl -. "DEFINED_IN" .-> F
    Tb -. "DEFINED_IN" .-> F

    %% --- Styling ---
    classDef file fill:#eceff1,stroke:#455a64,stroke-width:1px,stroke-dasharray: 5 5;
    classDef code fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;
    classDef state fill:#ffebee,stroke:#c62828,stroke-width:2px;
    classDef db fill:#fff3e0,stroke:#e65100,stroke-width:2px;

    class F file;
    class Cl,Fn,Fd code;
    class Gl state;
    class Tb db;
```

---

## 3. Node Types and Property Reference

| Node Label | Description | Key Properties |
| :--- | :--- | :--- |
| `File` | Physical source file on disk. | `id` (relative path), `path`, `extension`, `lines`, `change_frequency` |
| `Class` | Class, struct, record, or module. | `id`, `name`, `file`, `start_line`, `end_line`, `is_interface` |
| `Interface` | Interface definition (Java, C#, TypeScript). | `id`, `name`, `file`, `start_line`, `end_line` |
| `Function` | Function, method, or procedure definition. | `id`, `name`, `file`, `start_line`, `end_line`, `signature`, `is_volatile`, `is_test`, `embedding` (768d), `fan_in`, `fan_out`, `volatility_score`, `risk_score` |
| `Constructor` | Class initialization constructor (Java, C#). | `id`, `name`, `file`, `start_line`, `end_line`, `signature` |
| `Field` | Member variable or class property. | `id`, `name`, `file`, `type`, `visibility` |
| `Global` | Global variable or static state variable. | `id`, `name`, `file`, `line`, `type` |
| `Table` | Relational database table parsed from SQL DDL. | `id`, `name`, `file`, `columns` (JSON string array) |

---

## 4. Edge Semantics & Behavioral Rules

### 4.1 Structural Edges
* `(:Class)-[:HAS_METHOD]->(:Function)`: Represents structural ownership. Indicates that the function or method is declared inside the class body.
* `(:Class)-[:DEFINES]->(:Field)`: Class ownership of member variables or fields.
* `(:Class)-[:EXTENDS]->(:Class)` / `[:IMPLEMENTS]->(:Interface)`: Object-oriented inheritance and interface fulfillment.
* `(:Class)-[:DEPENDS_ON]->(:Class)`: Type-level dependency, such as constructor parameter types (Dependency Injection) or member types.
* `(*)-[:DEFINED_IN]->(:File)`: Links every code element back to its physical origin file for fast filesystem slicing.

### 4.2 Invocation & Behavioral Edges
* `(:Function)-[:CALLS]->(:Function)`: Direct function or method invocation. Forms the primary directed call graph utilized by pathfinding, blast-radius queries, and community detection.
* `(:Function)-[:USES]->(:Field)`: Direct access to instance variables or class properties.
* `(:Function)-[:USES_GLOBAL]->(:Global)`: Represents code reading or mutating global state. Critical for finding hidden side-channel coupling between otherwise decoupled services.
* `(:Function)-[:TESTS]->(:Function)`: Links automated unit test functions to production functions.

---

## 5. Polyglot Language Extraction Matrix

The ingestion engine dispatches files across a concurrent worker pool, mapping extensions to specialized Tree-sitter parsers:

| Language | Exts | Tree-sitter Parser | Extracted Entities & Behaviors |
| :--- | :--- | :--- | :--- |
| **C / C++** | `.c`, `.cpp`, `.h`, `.hpp`, `.cc` | `tree-sitter-cpp` | Functions, constructors, function pointers, templates, structs, classes, namespaces, inheritance, and flat-symbol global variable access (`USES_GLOBAL`). |
| **C#** | `.cs` | `tree-sitter-c-sharp` | Classes, interfaces, methods, properties, fields, constructor Dependency Injection (`DISupport`), inheritance, namespace hierarchy. |
| **Java** | `.java` | `tree-sitter-java` | Classes, interfaces, methods, annotations, constructors, inheritance (`EXTENDS`, `IMPLEMENTS`), class fields. |
| **TypeScript / JS**| `.ts`, `.tsx`, `.js`, `.jsx` | `tree-sitter-typescript` | Functions, arrow functions, classes, interfaces, method calls, ES6 imports/exports. |
| **Python** | `.py` | `tree-sitter-python` | Sync & async functions, decorators, class inheritance, method calls, type annotations, global variable assignments. |
| **VB.NET / ASP** | `.vb`, `.asp`, `.inc` | Custom Tree-sitter regex / grammar | Subroutines, functions, classes, module-level variables, end-line boundaries. |
| **SQL** | `.sql` | `tree-sitter-sql` | `CREATE TABLE`, `CREATE VIEW`, primary keys, foreign keys, table-to-table joins. |

### Technical Boundary: AST vs. Full Compiler Frontend
The ingestion pipeline relies on Tree-sitter (syntactic analysis) rather than a complete compiler frontend (e.g., Roslyn, Clang LibTooling, or Java LSP):
* **Why Tree-sitter?** It parses broken or incomplete legacy code in milliseconds, requires zero project build files (`.csproj`, `CMakeLists.txt`, `pom.xml`), operates in an unconfigured environment, and runs completely offline.
* **Lexical Scope Trade-offs:** Accurately resolving method overloading and variable shadowing across deep inheritance chains in Java/TypeScript (e.g., differentiating local `count` from `this.count` or resolving dynamic runtime dispatch) is impossible with syntactic ASTs alone. 
* **Symbol Matching Heuristics:** The engine resolves cross-file invocations using qualified symbol matching within the active file's import declarations and global symbol namespace.

---

## 6. Incremental Ingestion & Database Sync

When analyzing large repositories during active development, re-parsing hundreds of thousands of unchanged files is wasteful:
1. `graphdb ingest -since-commit <HASH>` queries Git for changed files between `<HASH>` and `HEAD`.
2. Unchanged files are skipped entirely.
3. Changed files are re-parsed into an in-memory `Neo4jEmitter`.
4. Stale nodes and edges belonging to the modified files are purged from Neo4j, and updated entities are upserted atomically in a single Cypher transaction.

[^ingest-walker]: [`walker.go`](file:///home/jasondel/dev/graphdb-skill/internal/ingest/walker.go)
[^analysis-base]: [`types.go`](file:///home/jasondel/dev/graphdb-skill/internal/analysis/types.go)
[^neo4j-loader]: [`neo4j_loader.go`](file:///home/jasondel/dev/graphdb-skill/internal/loader/neo4j_loader.go)
