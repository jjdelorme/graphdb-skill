# Research Report: RPG Implementation Status

**Date:** February 11, 2026
**Target:** `internal/rpg/` and usage in `cmd/graphdb/main.go`
**Objective:** Determine if the RPG implementation is complete or relies on placeholders.

## 1. Executive Summary

The current implementation of the RPG (Repository Planning Graph) in the Go codebase is **incomplete and heavily reliant on stubs/placeholders**. While the structural interfaces (`DomainDiscoverer`, `Clusterer`, `Summarizer`) and data models (`Feature`, `Builder`, `Enricher`) are defined in `internal/rpg/`, the actual logic to populate and persist the graph is missing or mocked.

**Status:** 🚧 **Skeleton / Prototype Only**

## 2. Key Findings

### A. Explicit Placeholders in `cmd/graphdb/main.go`
The `main.go` file contains explicit placeholder implementations that bypass real logic:

1.  **`SimpleDomainDiscoverer`**:
    ```go
    // Placeholder: returns a single root domain
    return map[string]string{"root": ""}, nil
    ```
    *   *Impact:* No actual domain discovery occurs; the entire codebase is treated as a single "root" domain.

2.  **`SimpleClusterer`**:
    ```go
    // Placeholder: puts all nodes in a single "default" cluster
    return map[string][]graph.Node{"default": nodes}, nil
    ```
    *   *Impact:* No clustering (semantic or structural) is performed.

3.  **`MockSummarizer`**:
    ```go
    return "Mock Feature", "Automatically generated description...", nil
    ```
    *   *Impact:* No LLM-based summarization or enrichment.

### B. Incomplete Logic in `internal/rpg/builder.go`
The `Builder.Build` method contains comments explicitly admitting to missing functionality:
```go
// In a real impl, we would associate the 'nodes' (Functions) 
// with this 'child' (Feature) via "IMPLEMENTS" edges.
// For this structure-building phase, we just create the hierarchy.
domainFeature.Children = append(domainFeature.Children, child)
_ = nodes // Nodes are ignored
```
*   *Critical Gap:* The connection between the Intent Layer (`Feature` nodes) and the Dependency Layer (`Function` nodes) is **not created**. The `nodes` returned by the clusterer are discarded.

### C. Naive Logic in `internal/rpg/enrich.go`
The enrichment logic is rudimentary:
```go
// Truncate to save tokens?
if len(content) > 200 {
    snippets = append(snippets, content[:200]+"...")
}
```
*   *Gap:* 200 characters is likely insufficient for meaningful summarization of complex functions.

### D. Lack of Persistence
The `enrich-features` command in `main.go` only outputs JSON to `stdout`. It **does not emit nodes or edges** to the graph storage (Neo4j or JSONL), meaning the RPG is transient and cannot be queried later.

## 3. Dependency Map

*   **`internal/rpg`**:
    *   Imports: `graphdb/internal/graph` (Core models), `strings`.
    *   Imported By: `cmd/graphdb` (Main entry point).
    *   *Isolation:* The package is well-isolated. It does not depend on `ingest` or `storage`.

*   **`internal/ingest`**:
    *   **Does NOT import `internal/rpg`**. The ingestion pipeline (parsing, embedding) is completely decoupled from the RPG construction.

## 4. Recommendations

1.  **Implement Real Discoverers:** Replace `SimpleDomainDiscoverer` with a heuristic-based (directory structure) or LLM-based discoverer.
2.  **Implement Real Clusterers:** Replace `SimpleClusterer` with a semantic clustering implementation (using embeddings from `internal/embedding`).
3.  **Wire up Persistence:** Update `Builder` to return `[]graph.Edge` (specifically `IMPLEMENTS` edges) and modify `main.go` to emit these to the storage layer.
4.  **Integrate LLM Client:** Connect `Enricher` to a real LLM client (e.g., Vertex AI) instead of `MockSummarizer`.
5.  **Connect to Ingest:** Consider if RPG construction should happen *during* ingestion or as a post-processing step (currently modeled as post-processing `enrich-features`, which is appropriate).

## 5. Conclusion

The `rpg` package provides the **types and interfaces** but lacks the **implementation**. It is currently a test harness suitable for unit testing the architecture but not for production use.
