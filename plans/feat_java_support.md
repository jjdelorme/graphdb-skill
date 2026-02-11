# Feature Implementation Plan: Java Language Support

**Goal:** Extend the GraphDB skill to support parsing of Java source code using Tree-sitter.
**Context:** The current ingestion pipeline supports C#, C++, VB.NET, SQL, ASP, and TypeScript. Adding Java support will allow the skill to process Java repositories, extracting class and method definitions, and function call relationships.

## 📋 Todo Checklist
- [x] Create Java fixture for testing
- [x] Implement Java Parser with Tree-sitter bindings
- [x] Verify implementation with tests
- [x] Final Review and Testing

## 🔍 Analysis & Investigation

### Architecture
The `internal/analysis` package uses a plugin-like architecture where parsers register themselves via `init()` functions.
-   **Interface:** `LanguageParser` (Parse method returns Nodes and Edges).
-   **Registry:** `parsers` map in `parser.go`.
-   **Discovery:** `internal/ingest/worker.go` uses `analysis.GetParser(ext)` to dynamically select the correct parser.

### Dependencies
-   **Library:** `github.com/smacker/go-tree-sitter` is already used.
-   **Language Binding:** `github.com/smacker/go-tree-sitter/java` is available and compatible.

### Implementation Details
We need to map Java constructs to the GraphDB schema:
-   **Nodes:**
    -   `Class`, `Interface`, `Enum` -> `Class` (or similar label, though currently we mostly use `Class` and `Function`).
    -   `Method`, `Constructor` -> `Function`.
-   **Edges:**
    -   `CALLS`: Method invocations and object creations.

## 📝 Implementation Plan

### Prerequisites
-   Ensure `github.com/smacker/go-tree-sitter/java` is fetched (will be handled by `go mod tidy` or implicit download).

### Step-by-Step Implementation

#### Phase 1: Test Harness
1.  **Step 1.A (The Harness):** Create a comprehensive Java fixture.
    *   *Action:* Create `test/fixtures/java/sample.java`.
    *   *Content:* A Java file containing:
        -   A class with a method.
        -   An interface.
        -   A constructor.
        -   Method calls (internal and external).
        -   Object creation (`new`).
    *   *Goal:* Provide a ground truth for testing the parser.

2.  **Step 1.B (The Test):** Create the test file.
    *   *Action:* Create `internal/analysis/java_test.go`.
    *   *Content:*
        -   Test registration (`GetParser(".java")`).
        -   Test parsing of `sample.java`.
        -   Assert existence of specific `Function` and `Class` nodes.
        -   Assert existence of `CALLS` edges.

#### Phase 2: Implementation
1.  **Step 2.A (The Parser):** Implement the Java Parser.
    *   *Action:* Create `internal/analysis/java.go`.
    *   *Detail:*
        -   Import `github.com/smacker/go-tree-sitter/java`.
        -   Struct `JavaParser`.
        -   `init()`: Register for `.java`.
        -   `Parse()`:
            -   **Definition Query:**
                -   `class_declaration` -> `name`
                -   `interface_declaration` -> `name`
                -   `enum_declaration` -> `name`
                -   `record_declaration` -> `name` (if supported by grammar)
                -   `method_declaration` -> `name`
                -   `constructor_declaration` -> `name`
            -   **Reference Query:**
                -   `method_invocation` -> `name`
                -   `object_creation_expression` -> `type`
            -   **Helper:** `findEnclosingJavaFunction` to resolve source of calls.

2.  **Step 2.B (Verification):** Run the tests.
    *   *Action:* Run `go test -v internal/analysis/java_test.go`.
    *   *Success:* Test passes with all nodes and edges correctly identified.

### Testing Strategy
-   **Unit Tests:** verify that the parser correctly extracts nodes and edges from valid Java code.
-   **Integration:** implicitly tested by `go test ./internal/analysis` which runs all tests in the package.

## 🎯 Success Criteria
-   `internal/analysis/java.go` exists and compiles.
-   `internal/analysis/java_test.go` passes.
-   The parser correctly identifies:
    -   Classes
    -   Methods
    -   Constructors
    -   Function Calls
