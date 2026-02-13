# Feature Implementation Plan: Standardize Parser Models

## 📋 Todo Checklist
- [ ] **Standardize C# Parser**
    - [ ] Add `HAS_METHOD` edge generation.
    - [ ] Update Class IDs to be Global (unscoped).
- [ ] **Standardize TypeScript Parser**
    - [ ] Add `HAS_METHOD` edge generation.
    - [ ] Update Class IDs to be Global (unscoped).
- [ ] **Standardize C++ Parser**
    - [ ] Implement Class/Struct capture.
    - [ ] Add `HAS_METHOD` edge generation.
    - [ ] Update Class IDs to be Global (unscoped).
- [ ] **Standardize VB.NET Parser**
    - [ ] Refactor Regex logic to track Class scope.
    - [ ] Add `HAS_METHOD` edge generation.
    - [ ] Update Class IDs to be Global (unscoped).
- [ ] **Verification**
    - [ ] Create `test/model_consistency_test.go` to assert graph structure across all languages.

## 🔍 Analysis & Investigation

### Current State
The "Scout" agent identified inconsistencies in how different language parsers construct the dependency graph. 
*   **Java (Reference):** Uses **Global IDs** for classes (e.g., `MyClass`) and links methods to classes via `HAS_METHOD`. This enables cross-file dependency resolution (e.g., `FileA` calls `MyClass` defined in `FileB`).
*   **C# / TypeScript:** Use **File-Scoped IDs** for classes (e.g., `file.cs:MyClass`). This isolates the class definition, preventing cross-file linking unless an explicit resolver runs later (which currently doesn't exist). They also lack `HAS_METHOD` edges.
*   **C++:** Completely ignores Class/Struct definitions, only capturing loose functions or methods as functions.
*   **VB.NET:** Uses a legacy Regex parser that captures classes but creates `DEFINED_IN` edges to the *File*, not linking methods to the *Class*.

### The Standard Graph Model
To ensure consistent dependency analysis, all parsers must adhere to this model:

#### Nodes
| Node Type | ID Format | Example | Purpose |
| :--- | :--- | :--- | :--- |
| `Class` | `ClassName` | `CustomerService` | Represents the Type definition. Global ID allows linking across files. |
| `Function` | `FilePath:FunctionName` | `src/service.ts:login` | Represents the implementation block. Scoped to file to avoid method name collisions. |
| `File` | `FilePath` | `src/service.ts` | Represents the physical file. |

#### Edges
| Source | Relation | Target | Purpose |
| :--- | :--- | :--- | :--- |
| `Class` | `HAS_METHOD` | `Function` | Structural containment. Essential for traversing from a Type to its behavior. |
| `Function` | `CALLS` | `Function` | Direct invocation (intra-class or static). |
| `Function` | `CALLS` | `Class` | Constructor invocation (e.g., `new Class()`). |
| `Function` | `USES` | `Class` | Dependency injection or property access. |

## 📝 Implementation Plan

### Prerequisites
*   Ensure `test/fixtures/` contains sample code for C#, C++, TypeScript, and VB.NET that includes a Class with a Method.

### Phase 1: The Verification Harness
We need a unified test to ensure all parsers produce the standard model.

1.  **Step 1.A:** Create `test/model_consistency_test.go`.
    *   *Action:* Define a table-driven test that runs against fixtures for `.cs`, `.ts`, `.cpp`, `.vb`.
    *   *Assertion:*
        *   Finds a `Class` node with ID `ClassName` (no file prefix).
        *   Finds a `Function` node with ID `FilePath:MethodName`.
        *   Asserts an edge `(Class) -[HAS_METHOD]-> (Function)` exists.

### Phase 2: C# Standardization
1.  **Step 2.A:** Verify Failure.
    *   *Action:* Run the new consistency test. Expect failure (Scoped IDs, missing edges).
2.  **Step 2.B:** Update `internal/analysis/csharp.go`.
    *   *Action:* Modify the Tree-sitter query loop.
    *   *Detail:*
        *   When capturing `class_declaration`, set `ID = nodeName` (remove file prefix).
        *   Identify the parent Class for each Method.
        *   Create `HAS_METHOD` edge from Class Global ID to Method Scoped ID.
3.  **Step 2.C:** Verify Success.
    *   *Action:* Run the test.

### Phase 3: TypeScript Standardization
1.  **Step 3.A:** Verify Failure.
2.  **Step 3.B:** Update `internal/analysis/typescript.go`.
    *   *Action:* Modify the Tree-sitter query loop.
    *   *Detail:*
        *   Update `class_declaration` / `interface_declaration` to use Global ID.
        *   Implement `findEnclosingClass` helper (similar to Java parser).
        *   Generate `HAS_METHOD` edges.
3.  **Step 3.C:** Verify Success.

### Phase 4: C++ Standardization
1.  **Step 4.A:** Verify Failure.
2.  **Step 4.B:** Update `internal/analysis/cpp.go`.
    *   *Action:* Update Tree-sitter query.
    *   *Detail:*
        *   Add `class_specifier` and `struct_specifier` to the definition query.
        *   Capture Class names and create Class nodes with Global IDs.
        *   Update method parsing to identify the enclosing class (or namespace).
        *   Generate `HAS_METHOD` edges.
3.  **Step 4.C:** Verify Success.

### Phase 5: VB.NET Standardization
1.  **Step 5.A:** Verify Failure.
2.  **Step 5.B:** Update `internal/analysis/vbnet.go`.
    *   *Action:* Refactor the line-by-line Regex loop.
    *   *Detail:*
        *   Maintain a `currentClass` state variable.
        *   When a Class is found, emit a Class node with Global ID.
        *   When a Function is found, if `currentClass` is set, emit `HAS_METHOD` edge.
        *   Ensure `End Class` resets the `currentClass` state.
3.  **Step 5.C:** Verify Success.

### Testing Strategy
*   **Unit Tests:** existing `internal/analysis/*_test.go` files should be updated to reflect the new ID schema (they will likely break when IDs change).
*   **Integration Test:** The new `test/model_consistency_test.go` will be the primary gatekeeper.

## 🎯 Success Criteria
*   All 4 parsers (C#, TS, C++, VB) pass `test/model_consistency_test.go`.
*   Generated graphs show `Class` nodes as "Hubs" connected to their methods via `HAS_METHOD`.
*   Cross-file dependencies (simulated by matching Global IDs) are theoretically possible (proven by the shared ID schema).
