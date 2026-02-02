# Plan: GraphDB Skill Generalization

**Objective:** Transform the `graphdb` skill from a project-specific (Alpine/Plating) tool into a reusable Gemini CLI skill applicable to any codebase containing C++, C#, VB.NET, ASP, or SQL.

## 1. Analysis of Current State

### 1.1 Project-Specific Artifacts
*   **Hardcoded Paths:**
    *   `tools/graph_extraction/extract_graph_v2.js`: Hardcoded `VIEW/` and `ODA_CAD` filters.
    *   `scripts/import_to_neo4j.js`: Hardcoded relative path to `../../../../graph_data`.
*   **Hardcoded Credentials:**
    *   Multiple files default to `Alpine123!`.
*   **Specific Logic:**
    *   `.gemini/skills/graphdb/scripts/check_plating.js`: Entire file is specific to the "plating" module.
    *   `GENERATE_GRAPH_INSTRUCTIONS.md`: Contains Alpine-specific workflow steps.
*   **Directory Structure:**
    *   Critical extraction logic lives in `tools/graph_extraction/` (outside the skill).
    *   Data is stored in project root `graph_data/`.

## 2. Generalization Strategy

### 2.1 Consolidation & Renaming
Move the extraction logic inside the skill to make it self-contained.
*   **Action:** Move `tools/graph_extraction/` -> `.gemini/skills/graphdb/extraction/`.
*   **Action:** Rename `extract_graph_v2.js` -> `extract_graph.js`.
*   **Action:** Update `package.json` in the skill to include extraction dependencies.

### 2.2 Configuration & Environment
The skill must be "zero-config" for standard use cases.
*   **Neo4j Credentials:** Strictly driven by `.env` file (utilizing `dotenv`). No hardcoded fallbacks.
    *   `NEO4J_URI`
    *   `NEO4J_USER`
    *   `NEO4J_PASSWORD`
*   **File Discovery:** The extractor will automatically scan for supported file extensions from the project root.
    *   **C/C++:** `.c`, `.cc`, `.cpp`, `.cxx`, `.h`, `.hh`, `.hpp`, `.hxx`, `.inl`
    *   **C#:** `.cs`
    *   **VB.NET:** `.vb`
    *   **Web:** `.asp`, `.aspx`, `.cshtml`, `.razor`
    *   **Database:** `.sql`
    *   Users do NOT provide include/exclude lists manually unless strictly necessary (optional override).
    *   Standard ignore files (`.gitignore`, `.geminiignore`) must be respected.
*   **Compile Commands:** The path to `compile_commands.json` will not be static. The skill will dynamically generate or locate this file at runtime (e.g., using `generate_compile_commands.js` logic) to ensure up-to-date parsing for C++.

### 2.3 Testing Strategy (STRICT)
**Mandate:** All changes must follow a Test-First / TDD approach.
1.  **Snapshot/Unit Tests First:** Before moving or editing `extract_graph_v2.js`, create a test suite that verifies its current output/behavior given a sample input.
2.  **Refactor & Verify:** Move the file and rename it. Run tests to ensure they still pass.
3.  **Feature Implementation:**
    *   Write a failing test for "No Hardcoded Filters" (e.g., ensure it finds a file outside `VIEW/`). -> Fix code -> Pass.
    *   Write a failing test for "Environment Config" (mocking process.env). -> Fix code -> Pass.
    *   Write a failing test for "Auto-Discovery" of ASP/VB files. -> Fix code -> Pass.

## 3. Execution Steps

1.  **Test Harness Setup:**
    *   Create a dedicated test file `tools/graph_extraction/test/extract_graph.test.js` (before moving).
    *   Implement tests to cover current hardcoded behavior (proving the current limitations).

2.  **Consolidation:**
    *   Move `tools/graph_extraction/` contents to `.gemini/skills/graphdb/extraction/`.
    *   Rename `extract_graph_v2.js` to `extract_graph.js`.
    *   Verify tests pass in the new location.

3.  **Refactoring (Iterative TDD):**
    *   **Remove Hardcoded Paths:** Update `extract_graph.js` to rely on recursive directory scanning for supported extensions. Test against a dummy file structure.
    *   **Env Var Integration:** Update `import_to_neo4j.js` and `extract_graph.js` to use `dotenv`. Remove `Alpine123!`.
    *   **Dynamic Compile Commands:** Integrate logic to generate or find `compile_commands.json` on the fly.

4.  **Cleanup:**
    *   Delete `.gemini/skills/graphdb/scripts/check_plating.js`.
    *   Update `SKILL.md` to reflect the new setup (setting up `.env` instead of editing scripts).
    *   Update `package.json` dependencies.