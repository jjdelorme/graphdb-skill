# Feature Implementation Plan: Systemic Dependency Resolution

## 📋 Todo Checklist
- [x] **Phase 1: Java Import & Resolution** (Refined from previous plan)
- [x] **Phase 2: TypeScript Import & Path Resolution** ✅ Implemented
- [x] **Phase 3: C# Using & Namespace Resolution** ✅ Implemented
- [x] **Phase 4: C++ Include & Header Resolution** ✅ Implemented
- [ ] **Phase 5: System Verification (Multi-language Integration)**
- [ ] Final Review and Testing

## 🔍 Analysis & Investigation
The current parsers (`java.go`, `csharp.go`, `typescript.go`, `cpp.go`) rely on "Token Matching" (linking calls to variable names) rather than "Semantic Linking" (linking to definitions). This causes "Ghost Nodes" (references to non-existent nodes like variable names) and misses the actual dependencies defined in imports.

### The "Ghost Node" Problem
*   **Current State:** Call `userService.save()` -> Links to `Node(ID="userService")`.
*   **Issue:** `userService` is a local variable. The graph is disconnected from the actual `UserService` class.
*   **Desired State:** Call `userService.save()` -> Links to `Node(ID="src/services/UserService.ts:UserService")` (or equivalent).

### The Strategy: "Import-Inferred Linking"
We will implement a **Stateless Resolution Strategy**. We do not need a global symbol table during parsing. Instead, we use the file's own imports to deterministically predict the Target ID of a dependency.

**Formula:** `TargetID = Resolve(ImportPath, SymbolName)`

## 📝 Implementation Plan

### Prerequisites
*   Existing parsers in `internal/analysis/`.
*   `internal/graph/schema.go` (ensure Edge types support these new resolutions, mainly `USES` or `CALLS`).

---

### Phase 1: Java (Consolidated)
*Supersedes `feat_java_dependency_precision.md`.*

1.  **Step 1.A (The Harness):** Update `test/fixtures/java/sample.java` to include imports and dependency injection. Create `test/analysis/java_resolution_test.go`.
2.  **Step 1.B (Import Parsing):** Modify `internal/analysis/java.go`.
    *   Parse `import` statements. Map `Alias` -> `FullyQualifiedName` (e.g., `List` -> `java.util.List`).
3.  **Step 1.C (Local Resolution):**
    *   Track local variables/fields `varName -> TypeName`.
    *   On usage: `varName.method()` -> lookup `TypeName` -> lookup `FullyQualifiedName`.
    *   Generate ID: `FullyQualifiedName` (Logic: Java IDs are often class paths).

---

### Phase 2: TypeScript (Path Resolution)

#### Analysis
TypeScript linking requires resolving file paths. `import { User } from './models/User'` implies the target is at `currentDir/models/User.ts`.

#### Step 2.A: The Harness
1.  Create `test/fixtures/typescript/complex_imports.ts` and `test/fixtures/typescript/models/User.ts`.
2.  Create `test/analysis/typescript_resolution_test.go`.
    *   *Goal:* Assert that `import { User } ...; const u = new User();` creates an edge to `.../models/User.ts:User`.

#### Step 2.B: Import Parsing
1.  **Action:** Modify `internal/analysis/typescript.go`.
2.  **Logic:** Add Tree-Sitter query for imports:
    ```scheme
    (import_statement
      source: (string) @import.source
      (import_clause (named_imports (import_specifier name: (identifier) @import.name)))
    )
    ```
3.  **Store:** `map[string]string` (Alias -> SourcePath).
    *   Example: `User` -> `./models/User`.

#### Step 2.C: Path Resolver
1.  **Action:** Implement `resolveTSPath(currentFile, importPath) string`.
2.  **Logic:**
    *   If `importPath` starts with `.`: Combine `dirname(currentFile)` + `importPath`.
    *   Normalize (remove `..`).
    *   Append extensions (`.ts`, `.tsx`) if missing (conceptually, or just append `.ts` as default for ID generation).
    *   **Result:** `src/models/User.ts`.

#### Step 2.D: Edge Generation
1.  **Action:** On `new_expression` or `call_expression`:
    *   Identify the token (`User`).
    *   Look up in Import Map.
    *   If found: TargetID = `resolvedPath + ":" + token`.
    *   If not found (local or global): Fallback to current behavior (or better, check for local definition).

---

### Phase 3: C# (Namespaces & Usings)

#### Analysis
C# uses Namespaces. `using System.Collections;` makes `ArrayList` available.
TargetID Strategy: Since files don't strictly map to classes (partial classes), we will use **Logical IDs** for the edges, or **File-Inferred IDs** where possible.
*Strategy:* `Namespace.ClassName`.

#### Step 3.A: The Harness
1.  Create `test/fixtures/csharp/dependency.cs`.
2.  Create `test/analysis/csharp_resolution_test.go`.

#### Step 3.B: Using Parsing
1.  **Action:** Modify `internal/analysis/csharp.go`.
2.  **Query:**
    ```scheme
    (using_directive (qualified_name) @using.namespace)
    (using_directive (name_equals name: (identifier) @alias.name (qualified_name) @alias.target))
    ```
3.  **Store:** List of `Usings` and Map of `Aliases`.

#### Step 3.C: Resolution Logic
1.  On `object_creation_expression` (e.g., `new List<string>()`):
    *   Extract type name `List`.
    *   If defined locally: Link to local ID.
    *   If matches Alias: Link to Aliased ID.
    *   Else: Generate "Potential IDs" based on Usings.
        *   *Heuristic:* We can't know *which* using contains it without an index.
        *   *Compromise:* Create a **Logical Edge** to `UNRESOLVED:List` with a property `candidates: [System.Collections.List, MyLib.List]`.
        *   *Better Compromise (Stateless):* If simple name, link to `using_namespace.TypeName` if there is only one likely candidate, or default to `Global:TypeName`.
        *   **Selected Strategy:** Link to `TypeName` (unqualified) but add property `possible_namespaces`. *Or*, simpler: Just link to the **Type Name** node.
        *   *Refined Strategy:* Identify explicit matches. For `var x = new My.Namespace.Class()`, link to `My.Namespace.Class`. For `using My.Namespace; ... new Class()`, link to `My.Namespace.Class`.

---

### Phase 4: C++ (Headers & Includes)

#### Analysis
C++ is the hardest. `#include "foo.h"` inserts content.
*Strategy:* Treat Header as the "Module".
*TargetID:* `path/to/foo.h:SymbolName`.

#### Step 4.0: Enhanced Extraction (Inheritance, Globals & Members)
1.  **Inheritance:** Capture `base_class_clause` -> `INHERITS` edge.
2.  **Members:** Capture field declarations in classes/structs -> `Field` nodes.
3.  **Globals:** Capture global variable declarations.
4.  **Usage:** Capture identifier references (reads/writes) -> `USES` edges (Critical for legacy global state analysis).

#### Step 4.A: The Harness
1.  Create `test/fixtures/cpp/main.cpp` and `test/fixtures/cpp/math.h`.
2.  Create `test/analysis/cpp_resolution_test.go`.

#### Step 4.B: Include Parsing
1.  **Action:** Modify `internal/analysis/cpp.go`.
2.  **Query:**
    ```scheme
    (preproc_include path: (string_literal) @include.path)
    (preproc_include path: (system_lib_string) @include.system)
    ```

#### Step 4.C: Heuristic Resolution
1.  **Logic:**
    *   When `MyClass` is used:
    *   Check if defined in current file.
    *   If not, look at includes.
    *   *Heuristic:* If `MyClass` matches `MyClass.h` (case-insensitive), assume it comes from there.
    *   *Fallthrough:* If unable to determine, link to `UNKNOWN_HEADER:MyClass`.

### Testing Strategy
*   **Unit Tests:** Each language parser has its own `_test.go` verifying the specific resolution logic (Paths for TS, Namespaces for C#).
*   **Integration:** Run `graphdb ingest` on `test/fixtures/` and inspect `output/graph.jsonl`.
*   **Verification:** Check that `CALLS` edges now point to `TargetID`s that resemble file paths or fully qualified names, not just simple names.

## 🎯 Success Criteria
1.  **Java:** Imports resolve to Full Class Names.
2.  **TypeScript:** Imports resolve to Absolute File Paths.
3.  **C#:** Usings resolve to Namespaced Types.
4.  **C++:** Includes resolve to Header Paths (best effort).
5.  **Ghost Nodes Eliminated:** No more edges pointing to `varName`.
