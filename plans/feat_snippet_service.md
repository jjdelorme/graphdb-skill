# Feature Implementation Plan: Snippet Service Extraction

## 📋 Todo Checklist
- [x] **Phase 1: Snippet Service Implementation**
    - [x] Create `internal/tools/snippet` package structure.
    - [x] Implement `SliceFile` logic.
    - [x] Implement `FindPatternInScope` logic.
    - [x] Add comprehensive unit tests in `snippet_test.go`.
- [x] **Phase 2: Refactoring Neo4j Provider**
    - [x] Refactor `FetchSource` in `internal/query/neo4j.go` to use `snippet.SliceFile`.
    - [x] Refactor `LocateUsage` in `internal/query/neo4j.go` to use `snippet.SliceFile` and `snippet.FindPatternInScope`.
    - [x] Align `LocateUsage` output format with legacy `SnippetService.js` (parity).
- [x] **Phase 3: Verification**
    - [x] Run `go test ./internal/tools/snippet/...`
    - [x] Verify `FetchSource` and `LocateUsage` functionality (manual or integration test if possible).

## 🔍 Analysis & Investigation
### Current State
- **File Slicing:** Logic embedded in `FetchSource` (opens file, scans lines, filters range).
- **Pattern Matching:** Logic embedded in `LocateUsage` (opens file, scans lines, filters range + `strings.Contains`).
- **Legacy Parity:** The legacy `SnippetService.js` provided reusable methods `sliceFile` and `findPatternInScope` (with context support). The current Go implementation lacks context support in `LocateUsage` and duplicates file reading logic.

### Objectives
1.  **Modularity:** Centralize file operations in `internal/tools/snippet`.
2.  **Parity:** Replicate `SnippetService.js` features, specifically `FindPatternInScope` with context lines.
3.  **Clean Code:** Remove duplication in `internal/query/neo4j.go`.

### Dependencies
- Standard Library: `os`, `bufio`, `strings`.
- No external dependencies for the snippet service itself.

## 📝 Implementation Plan

### Prerequisites
- Create directory `internal/tools/snippet`.

### Step-by-Step Implementation

#### Phase 1: Snippet Service Implementation
1.  **Step 1.A (The Harness):** Create the test file first to define behavior.
    *   *Action:* Create `internal/tools/snippet/snippet_test.go`.
    *   *Goal:* Define test cases for `SliceFile` (valid range, out of bounds, file not found) and `FindPatternInScope` (pattern found, context lines, no match).
2.  **Step 1.B (The Implementation):** Implement the service.
    *   *Action:* Create `internal/tools/snippet/snippet.go`.
    *   *Detail:*
        ```go
        package snippet

        type Line struct {
            Number  int    `json:"number"`
            Content string `json:"content"`
        }

        type Match struct {
            Lines []Line `json:"lines"`
        }

        // SliceFile reads a file and returns lines between start and end (inclusive, 1-based).
        func SliceFile(path string, startLine, endLine int) (string, error) { ... }

        // FindPatternInScope searches content for pattern and returns matches with context.
        // startLineOffset is the line number of the first line in content (usually 1 or the slice start).
        func FindPatternInScope(content, pattern string, contextLines int, startLineOffset int) ([]Match, error) { ... }
        ```
3.  **Step 1.C (The Verification):**
    *   *Action:* Run `go test ./internal/tools/snippet/...`.
    *   *Success:* All tests pass.

#### Phase 2: Refactoring Neo4j Provider
1.  **Step 2.A (The Harness):** Setup integration/verification baseline.
    *   *Action:* Create a temporary test in `internal/query/neo4j_test.go` or rely on `snippet` unit tests if integration is too complex (Neo4j dependency). *Decision:* Rely on unit tests for logic, and code review for wiring.
2.  **Step 2.B (The Refactor):** Modify `internal/query/neo4j.go`.
    *   *Action:* Import `graphdb/internal/tools/snippet`.
    *   *Update `FetchSource`:* Replace custom file reading with `snippet.SliceFile`.
    *   *Update `LocateUsage`:*
        *   Call `snippet.SliceFile` to get the target file segment.
        *   Call `snippet.FindPatternInScope` with `contextLines: 0` (or `2` for parity upgrade). *Decision:* Default to `0` for now to minimize noise, but return the structured `Match` objects.
        *   Return `[]Match` (which satisfies `any`).
3.  **Step 2.C (The Verification):**
    *   *Action:* Compile and ensure no build errors.
    *   *Success:* `make build` passes.

### Testing Strategy
- **Unit Tests:** Extensive testing of `internal/tools/snippet` covering edge cases (empty files, start > end, pattern not found).
- **Integration Tests:** Existing Neo4j tests should continue to pass.

## 🎯 Success Criteria
- `internal/tools/snippet` package exists and is tested.
- `internal/query/neo4j.go` is cleaner and delegates to the new package.
- `LocateUsage` returns structured data compatible with `SnippetService.js` (conceptually).
