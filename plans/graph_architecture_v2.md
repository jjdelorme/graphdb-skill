# Plan: Next-Generation Graph Extraction Architecture

**Objective:** Upgrade the current ad-hoc C++ graph extractor into a robust, extensible platform capable of analyzing multi-language codebases (initially C++ and C#) with precise global state tracking.

**Goal:** Transform the "Global Analysis" workflow from a fragile manual process into a reliable, automated standard operating procedure.

## 1. Core Architecture: The Adapter Pattern

We will decouple the **Graph Builder** (Core) from the **Language Parsers** (Adapters).

### 1.1 The Core (`GraphBuilder`)
*   **Responsibility:** Manages the 2-pass analysis loop, node/edge generation, unique ID assignment, and file I/O.
*   **Agnosticism:** Does NOT know about `tree-sitter` node types or language keywords. It operates on abstract `Symbol` objects.
*   **Workflow:**
    1.  **Pass 1 (Discovery):** Iterates files -> Calls Adapter -> Registers `Definitions` (Global Variables, Functions, Classes).
    2.  **Pass 2 (Linkage):** Iterates files -> Calls Adapter -> Resolves `References` (Calls, Variable Usages) against the registry from Pass 1.
    3.  **Output:** Writes normalized `nodes.json` and `edges.json`.

### 1.2 The Adapter Interface
Every supported language must implement this interface:

```javascript
interface LanguageAdapter {
    /**
     * Initializes parser (loads WASM, etc.)
     */
    init(): Promise<void>;

    /**
     * Parses a file and returns a standard AST wrapper or raw tree
     */
    parse(sourceCode: string): Tree;

    /**
     * Pass 1: Identify Definitions
     * Scans the tree to find top-level symbols.
     * Returns a list of: { name, type: 'Function'|'Class'|'Global'|'FileStatic'|'Constant', line }
     */
    scanDefinitions(tree: Tree): Definition[];

    /**
     * Pass 2: Identify References
     * Scans a specific scope (function body) for outbound dependencies.
     * Handles local scoping/shadowing internally.
     * Returns: { sourceFunction, referencedName, type: 'Call'|'Usage' }
     */
    scanReferences(tree: Tree, knownGlobals: Set<string>): Reference[];
}
```

## 2. Language Adapters

### 2.1 C++ Adapter (`CppAdapter`)
*   **Parsing:** Uses `tree-sitter-cpp`.
*   **Heuristics (Refined):**
    *   **Globals:** `declaration` nodes at `translation_unit` level.
    *   **File Statics:** Top-level declarations with `storage_class_specifier` containing `static`.
    *   **Constants:** Top-level declarations with `type_qualifier` containing `const` or `#define` macros.
    *   **Scope:** Handles namespaces (`::`) effectively.

### 2.2 C# Adapter (`CsharpAdapter`)
*   **Parsing:** Uses `tree-sitter-c-sharp`.
*   **Semantics:**
    *   **Globals:** `static` fields on `public` classes (effectively global state in .NET).
    *   **Dependency Injection:** Detects constructor injection patterns (future goal).
    *   **Properties:** Treats Property Getters/Setters as methods/usages.

## 3. Infrastructure Improvements

### 3.1 Robust Import Script (`import_to_neo4j.js`)
*   **Batched Deletes:** Permanently replace `MATCH (n) DETACH DELETE n` with `CALL { ... } IN TRANSACTIONS`.
*   **Resiliency:** Add retry logic for connection timeouts.
*   **Config:** Support `.env` file for host/password configuration (avoiding hardcoded creds).

### 3.2 Analysis Queries
*   **Standardized Scripts:** Port the ad-hoc `analyze_globals.js` logic into the main `query_graph.js` tool.
*   **New Command:** `analyze-state --function <name>`
    *   Returns categorization: `MutableGlobals`, `FileStatics`, `Constants` automatically.

## 4. Execution Roadmap

### Phase 1: Refactoring the Core (C++ Only) - ✅ COMPLETED
1.  ✅ Extract current C++ logic from `extract_graph.js` into `adapters/CppAdapter.js`.
2.  ✅ Rewrite `extract_graph.js` to use the Adapter Interface (Created `extract_graph_v2.js`).
3.  ✅ Implement the "Storage Class" distinction (Global vs. Static vs. Const) in the C++ adapter.
4.  ✅ Verify against codebase to ensure generation of `nodes.json` and `edges.json`.

### Phase 2: Adding C# Support - ✅ COMPLETED
1.  ✅ Install `tree-sitter-c-sharp`.
2.  ✅ Create `adapters/CsharpAdapter.js` implementing the `LanguageAdapter` interface.
3.  ✅ Update `extract_graph_v2.js` to register the C# adapter.
4.  ✅ Run against the `.NET` portion of the Alpine codebase.
5.  **Integration:** Connected the graph. Cross-language edges are implicitly possible via node lookups.

### Phase 3: Infrastructure Hardening - ✅ COMPLETED
1.  ✅ Update import script (`import_to_neo4j.js`) with batched deletes, retry logic, and `.env` support.
2.  ✅ Add `analyze-state` query type to `query_graph.js` and ensure it also supports `.env`.

## 5. Artifacts
*   `tools/graph_extraction/core/GraphBuilder.js` (Created)
*   `tools/graph_extraction/adapters/CppAdapter.js` (Created)
*   `tools/graph_extraction/adapters/CsharpAdapter.js` (Created)
*   `tools/graph_import/import_to_neo4j.js` (Refactored)
*   `tools/graph_import/query_graph.js` (Refactored)
