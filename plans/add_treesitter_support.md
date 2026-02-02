# Plan: Add Tree-sitter Support for VB.NET, ASP.NET, & Legacy ASP

## Goal
Extend the graph extraction tool (`tools/graph_extraction`) to support:
1.  **C#** (Verify/Enhance existing support).
2.  **VB.NET** (New Adapter).
3.  **ASP.NET** (WebForms `.aspx`, Razor `.cshtml`) - via embedded code extraction.
4.  **Legacy ASP** (`.asp`) - via embedded code extraction and VBScript parsing.

## Status Overview (Updated Feb 2, 2026)

| Language | Adapter Status | Test Status | Notes |
| :--- | :--- | :--- | :--- |
| **C#** | ✅ Existing | ✅ Verified | Baseline functionality verified. Converted to Native bindings. |
| **C++** | ✅ Existing | ✅ Verified | Converted to Native bindings. `CppAdapter.test.js` created. |
| **VB.NET** | ✅ Implemented | ✅ Verified | `VbAdapter.js` created and verified. |
| **ASP.NET** | ✅ Implemented | ✅ Verified | `AspAdapter.js` verified. Supports `.aspx` (C#/VB) and `.asp` (Legacy VBScript). |
| **SQL** | ✅ Completed | ✅ Verified | Robust Regex implementation used; `SqlAdapter.test.js` passing. |

## Technical Note: Native vs WASM Refactor
To ensure compatibility and performance, all adapters (C#, C++, VB.NET) have been standardized to use **native** `tree-sitter` bindings instead of `web-tree-sitter` (WASM). This simplifies the build process and removes runtime path dependencies for WASM files.

## Legacy ASP Support
Legacy `.asp` files (Classic ASP) are now explicitly supported. The `AspAdapter` detects `.asp` extensions or `<%@ Language=VBScript %>` directives and processes the embedded code using the VB.NET parser (wrapped in a module container), ensuring VBScript logic is extracted.

## Analysis & Dependencies

| Language | Extension | Tree-sitter Grammar | npm Package | Strategy |
| :--- | :--- | :--- | :--- | :--- |
| **C#** | `.cs` | `tree-sitter-c-sharp` | `tree-sitter-c-sharp` (Installed) | Keep using `CsharpAdapter`. |
| **VB.NET** | `.vb` | `tree-sitter-vbnet` | `tree-sitter-vb-dotnet` | Create `VbAdapter`. |
| **ASP.NET** | `.aspx`, `.cshtml` | `tree-sitter-html` + `c-sharp`/`vb` | `tree-sitter-html` | Parse as HTML, extract code blocks, delegate to CS/VB adapter. |
| **Legacy ASP** | `.asp` | `tree-sitter-html` + `vbscript` | `tree-sitter-html`, `tree-sitter-vb-dotnet` (fallback) | Parse as HTML, extract blocks, try parsing with VB adapter or Regex fallback. |

*Note: A dedicated `tree-sitter-vbscript` package is not readily available on npm. We will attempt to use `tree-sitter-vb-dotnet` for VBScript best-effort parsing (as syntax is similar) or fall back to Regex for dependency extraction if parsing fails heavily.*

## Implementation Steps

### 1. Install Dependencies
Run the following in `tools/graph_extraction`:
```bash
npm install tree-sitter-vb-dotnet tree-sitter-html tree-sitter-sql
```

### 2. Create `VbAdapter.js`
Create a new adapter `tools/graph_extraction/adapters/VbAdapter.js`.
*   **Logic**: Similar to `CsharpAdapter`, but using `tree-sitter-vb-dotnet`.
*   **Definitions**: `ClassBlock`, `ModuleBlock`, `SubBlock`, `FunctionBlock`.
*   **References**: `InvocationExpression`, `MemberAccess`.

### 3. Create `SqlAdapter.js`
Create a new adapter `tools/graph_extraction/adapters/SqlAdapter.js`.
*   **Logic**:
    1.  Parse `.sql` files using `tree-sitter-sql`.
    2.  Identify Definitions:
        *   **Stored Procedures**: `create_procedure` or similar. Create `Function` nodes.
        *   **Triggers**: `create_trigger` or similar. Create `Trigger` nodes. Link them to the table they watch (`watches` edge).
    3.  **Table Extraction**:
        *   Inspect `FROM` clauses, `JOIN` clauses, `UPDATE` targets, `INSERT INTO` targets.
        *   Extract table names (qualified or unqualified).
    4.  **Column Extraction**:
        *   Inspect `SELECT` lists, `WHERE` clauses, `SET` clauses.
        *   Attempt to link columns to tables (though this is hard without schema, we can capture "potential" column usage).
    5.  **Reference Extraction**:
        *   **SP Calls**: Identify `EXEC`, `EXECUTE` statements. Create `Call` edges between Stored Procedures.
    6.  **Output**:
        *   `Function` nodes for Stored Procedures.
        *   `Trigger` nodes for Triggers.
        *   `Table` nodes for database tables.
        *   `Usage` edges from Stored Proc/Trigger -> Table.
        *   `Call` edges from Stored Proc/Trigger -> Stored Proc.

### 4. Create `AspAdapter.js`
Create a new adapter `tools/graph_extraction/adapters/AspAdapter.js`.
*   **Logic**:
    1.  Parse file using `tree-sitter-html`.
    2.  Traverse tree to find `<script>` tags or text nodes containing `<% ... %>`.
    3.  Extract the content.
    4.  Determine language (Default C# for `.cshtml`, look for `<%@ Page Language="..." %>` for `.aspx`).
    5.  Pass extracted text to `CsharpAdapter` or `VbAdapter` to get symbols.
    6.  Adjust line numbers of found symbols to match original file.

### 5. Update `extract_graph_v2.js`
*   Register new adapters (`VbAdapter`, `SqlAdapter`, `AspAdapter`).
*   Update file scanning (glob) to include `.vb`, `.aspx`, `.cshtml`, `.asp`, `.sql`.
*   Map extensions to adapters.

### 6. Verification
*   Create a simple test file for each language in `tools/parser_test/`.
*   Run the extractor and verify JSON output contains expected nodes and edges.

## Implementation Challenges & Deviations

### 1. Dependency Hell (Tree-sitter)
During implementation, a major version conflict occurred with `tree-sitter`:
*   **The Project Lock**: Existing adapters (C#, C++) depend on `tree-sitter` **v0.21.1**.
*   **Incompatible SQL Adapters**:
    *   `tree-sitter-sql` (npm v0.1.0) is too old (built with ancient ABI) and crashes at runtime with `Invalid language object`.
    *   `tree-sitter-sql` (GitHub/Modern) requires `tree-sitter` **v0.25.0+**.
*   **Blocking Upgrade**: Upgrading the core `tree-sitter` to v0.25.0+ is currently impossible because `tree-sitter-vb-dotnet`, `tree-sitter-c-sharp`, and `tree-sitter-cpp` do not yet support the new ABI.

### 2. Resolution Strategy
To bypass the "Dependency Hell" while maintaining stability:
*   **SQL Adapter**: Switched to a **Regex-based** implementation.
    *   *Why*: Removes the dependency entirely.
    *   *Trade-off*: Less precise than AST, but robust enough for top-level dependency extraction (Procedures, Tables, Calls).
*   **VB.NET Adapter**: Used native `tree-sitter-vb-dotnet` (v0.1.9) with `--legacy-peer-deps`.
*   **ASP.NET Adapter**: Implemented as planned (Masking + Delegation).
*   **Core**: Kept `tree-sitter` at v0.21.1 to preserve C# and C++ functionality.

## Technical Learnings (Regex & Escaping)
Implementing the Regex-based `SqlAdapter` programmatically revealed significant complexities in string escaping:
*   **The Double-Escape Chain**: Writing a Regex string to a file via a tool requires multiple layers of escaping.
    *   **Layer 1 (Regex Literal)**: To match a digit `\d`, the Regex string needs `\\d`.
    *   **Layer 2 (JS String)**: To represent `\\d` in a JS string variable, you need `\\\\d`.
    *   **Layer 3 (File Write)**: Depending on the tool's input handling, backslashes may be consumed, requiring up to `\\\\\\\\d` in the prompt to ensure `\\d` lands on disk.
*   **Solution**: Switched to `new RegExp()` constructor logic for clarity, but ultimately required extreme care with backslash counts (e.g., `\\\\\\\\s` was needed to produce `\\s` on disk, which JS reads as `\\s` string, which Regex compiles to `\s` whitespace token).
*   **Takeaway**: When generating code that contains complex Regex patterns, verify the file content on disk immediately, as "success" messages do not validate logical content.

## Next Steps: Quality Assurance
The implementation is complete but lacks a safety net. The reliance on manual verification during the "Escape Hell" phase highlights the critical need for automated tests.

### Goal: Create Permanent Test Suite
We must implement a persistent test suite in `tools/graph_extraction/test` to protect these adapters from regression and verify edge cases.

1.  **Framework**: Use Node.js built-in `node --test` or `mocha` (minimal dependency).
2.  **Scope**:
    *   **VB.NET**: Verify Class/Method/Inheritance extraction.
    *   **ASP.NET**: Verify Code Block extraction (CS vs VB) and line number mapping.
    *   **SQL**: (DONE) Verify extraction of Procs, Triggers, and Tables using the Regex engine (ensure no regression on escaping).
    *   **C#**: Verify baseline functionality.
3.  **Artifacts**:
    *   Create `tools/graph_extraction/test/fixtures/` with `.vb`, `.aspx`, `.sql` samples.
    *   Create `tools/graph_extraction/test/SqlAdapter.test.js` (Done).
    *   Create `tools/graph_extraction/test/VbAdapter.test.js`.
    *   Create `tools/graph_extraction/test/AspAdapter.test.js`.
