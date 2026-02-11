# Feature Implementation Plan: Phase 1 - The Go Ingestor

**Campaign:** The Go Ingestor (Campaign 1)
**Goal:** Build a high-performance, standalone Go binary to parse local repositories, generate the RPG (Repository Planning Graph), and output structured JSONL with strict parity to the legacy Node.js implementation.
**Context:** This CLI will replace the current `.gemini/skills/graphdb/extraction` scripts. It must match the existing output format exactly to ensure seamless integration with the existing Neo4j loader.

## 📋 Todo Checklist
- [x] **Core:** Initialize Go project with `Storage` interface abstraction.
- [x] **Storage:** Implement `JSONLEmitter` (Phase 1 implementation of Storage).
- [x] **Parity:** Implement parser for **C#**.
- [x] **Parity:** Implement parsers for **C/C++**.
- [x] **Parity:** Implement parser for **VB.NET**. ✅ Implemented
- [x] **Parity:** Implement parser for **SQL**.
- [x] **Parity:** Implement parser for **TypeScript**.
- [x] **Enrichment:** Integrate Vertex AI for embedding generation.
    - [x] **Client:** Implement Vertex AI client (`internal/embedding`).
    - [x] **Integration:** Wire into `ingest` worker.
- [x] **TDD:** Achieve 100% unit test coverage for parser logic.
- [x] **Verification:** "Golden Master" comparison between Node.js output and Go output.

## 🔍 Analysis & Investigation

### Architecture: The Storage Abstraction
To support the future migration to Spanner (Campaign 4) without rewriting core logic, we will define a strict interface for data output.

```go
// internal/storage/interface.go
type Emitter interface {
    EmitNode(node *graph.Node) error
    EmitEdge(edge *graph.Edge) error
    Close() error
}
```

*   **Phase 1:** `JSONLEmitter` implements this interface, writing to stdout/file in the format expected by `import_to_neo4j.js`.
*   **Phase 4:** `SpannerEmitter` will implement this to write directly to Google Cloud Spanner.

### Future Proofing: Query Support
While Phase 1 is focused on "Write" (Ingestion), the project structure must accommodate "Read" (Querying) for Phase 2.
*   **Action:** Ensure `internal/graph` defines the core data models (`Node`, `Edge`) in a way that is reusable by both `internal/ingest` (Phase 1) and `internal/query` (Phase 2).

### Parity Requirements
To achieve a successful migration, the Go Ingestor must be indistinguishable from the JS implementation in terms of data output and logic.

1.  **Vertex AI Integration:**
    *   The Ingestor must perform vector enrichment during or immediately after parsing, matching the logic in `enrich_vectors.js`.
    *   It must verify the presence of `GOOGLE_APPLICATION_CREDENTIALS` or `GCLOUD_ACCESS_TOKEN` and call the Vertex AI Embeddings API for `Function` nodes.

2.  **Graph Structure Parity:**
    *   **Nodes:** Must output `Function`, `File`, and `Global` labels with identical property sets (id, name, signature, content, embedding).
    *   **Edges:** Must output:
        *   `CALLS` (Function -> Function)
        *   `DEFINED_IN` (Function -> File)
        *   `USES_GLOBAL` (Function -> Global)

3.  **Output Compatibility:**
    *   The `JSONLEmitter` output must strictly adhere to the schema consumed by `import_to_neo4j.js`.
    *   **Decision:** We will stick to the existing JSONL format for Phase 1 to decouple Extraction (Go) from Loading (Node.js/Neo4j). This allows us to replace the Ingestor without yet rewriting the Loader.

### Language Parity Requirements
The current Node.js solution supports a specific set of languages. The Go version must support:
1.  **C#:** (via `tree-sitter-c-sharp`)
2.  **C/C++:** (via `tree-sitter-cpp`)
3.  **VB.NET:** (via `tree-sitter-vbnet` - *Risk: check availability/maturity*)
4.  **SQL:** (via `tree-sitter-sql`)
5.  **TypeScript:** (via `tree-sitter-typescript`)

*Note: Go support is NOT required for this phase as it is not in the legacy scope.*

## 📝 Implementation Plan

### Prerequisites
*   Go 1.21+ installed.
*   GCC/Clang installed (for CGO).
*   `go get github.com/smacker/go-tree-sitter`.
*   Access to legacy test files in `.gemini/skills/graphdb/extraction/test/`.
*   GCP Credentials for Vertex AI testing.

### Project Structure
```
/
├── cmd/
│   └── graphdb/
│       └── main.go           # Dependency injection (wires Emitter -> Walker)
├── internal/
│   ├── storage/
│   │   ├── interface.go      # Emitter interface
│   │   └── jsonl.go          # JSONL implementation
│   ├── analysis/             # Tree-sitter logic
│   │   ├── parser.go         # Generic parser interface
│   │   ├── csharp.go
│   │   ├── cpp.go
│   │   ├── vbnet.go
│   │   ├── sql.go
│   │   └── typescript.go
│   ├── embedding/            # NEW: Vertex AI Client
│   │   └── vertex.go         # Embedding generation
│   ├── graph/                # Data models (Shared with Phase 2)
│   │   └── schema.go         # Node/Edge structs
│   └── ingest/               # Orchestration
│       ├── walker.go         # File system traversal
│       └── worker.go         # Worker pool logic
├── test/
│   ├── e2e/                  # Binary execution tests
│   └── fixtures/             # Code snippets for each language
├── go.mod
└── go.sum
```

### Step-by-Step Implementation (Strict TDD)

#### Phase 1.1: Storage & Core Harness
1.  **Step 1.1.A (Red):** Define the Storage Interface.
    *   *Action:* Create `internal/storage/interface.go`.
    *   *Test:* Create `internal/storage/jsonl_test.go`. Define a test that initializes a `JSONLEmitter` and calls `EmitNode`.
    *   *Assert:* Test fails to compile (missing struct) or fails to run (missing method).
2.  **Step 1.1.B (Green):** ~~Implement JSONL Emitter.~~ ✅ Implemented
    *   *Action:* Create `internal/storage/jsonl.go`. Implement the writing logic to match the existing schema: `{"id": "...", "label": "..."}`.
    *   *Verify:* Run `go test ./internal/storage/...`. Green.
3.  **Step 1.1.C (Refactor):** ~~Ensure thread safety.~~ ✅ Implemented
    *   *Action:* Add `sync.Mutex` to the emitter if concurrent writes are expected. Update tests to run parallel emits.

#### Phase 1.2: The Parser Engine (Generic)
1.  **Step 1.2.A (Red):** ~~Define the Generic Parser logic.~~ ✅ Implemented
    *   *Test:* Create `internal/analysis/parser_test.go`. Test a mock parser that accepts a file path and returns dummy nodes.
    *   *Assert:* Fails.
2.  **Step 1.2.B (Green):** ~~Implement the Parser Interface & Registry.~~ ✅ Implemented
    *   *Action:* Create `internal/analysis/parser.go`. Define the `LanguageParser` interface and a factory method to select parsers by file extension.
    *   *Verify:* Run `go test ./internal/analysis`. Green.

#### Phase 1.3: Language Adapters (Iterative Parity)
*Repeat this cycle for: TypeScript, C#, C++, SQL, VB.NET*

1.  **Step 1.3.A (Red - The Harness):** Create language fixture.
    *   *Action:* Create `test/fixtures/[lang]/sample.[ext]`.
    *   *Test:* Create `internal/analysis/[lang]_test.go`.
    *   *Code:*
        ```go
        func TestParse[Lang](t *testing.T) {
            // Load fixture
            // Call parser
            // Assert: Expect specific function names/class names to be found
        }
        ```
    *   *Assert:* Test fails (Parser not implemented).
2.  **Step 1.3.B (Green - The Binding):** Implement Tree-sitter binding.
    *   *Action:* Create `internal/analysis/[lang].go`.
    *   *Detail:*
        *   Import `github.com/smacker/go-tree-sitter/[lang]`.
        *   Define query (S-expression) to extract `function_definition`, `class_definition`, etc.
        *   Map tree-sitter nodes to `graph.Node`.
    *   *Verify:* Run `go test ./internal/analysis -run TestParse[Lang]`.
3.  **Step 1.3.C (Refactor):** Optimize queries.
    *   *Action:* Clean up S-expressions. Ensure error handling for syntax errors in source files.

#### Phase 1.4: Embedding Integration
1.  **Step 1.4.A (Red):** Define Embedding Interface.
    *   *Test:* Create `internal/embedding/vertex_test.go` (mocked).
    *   *Assert:* Fails.
2.  **Step 1.4.B (Green):** Implement Vertex Client.
    *   *Action:* Create `internal/embedding/vertex.go`. Implement call to Google Vertex AI API.
    *   *Action:* Update `ingest/worker.go` to call embedding service for each `Function` node before emission.
    *   *Verify:* Mock test passes.

#### Phase 1.5: Orchestration & End-to-End
1.  **Step 1.5.A (Red):** Integration Test.
    *   *Test:* Create `test/e2e/walker_test.go`. Point it at `test/fixtures/`.
    *   *Assert:* Expect JSONL output in buffer.
2.  **Step 1.5.B (Green):** Wire Main.
    *   *Action:* Implement `cmd/graphdb/main.go`.
    *   *Logic:* Parse flags -> Init `JSONLEmitter` -> Init `VertexClient` -> Init `Walker` -> Run -> Close.
    *   *Verify:* Run `go run ./cmd/graphdb`.

### Test Plan: Parity Verification

To ensure we can hot-swap the Node.js ingestor with the Go ingestor, we will use a **Golden Master** approach.

1.  **Baseline Generation:**
    *   Run the **existing** Node.js extraction on a reference repo (e.g., this repo itself).
    *   Save output to `test/goldens/legacy_output.jsonl`.
2.  **Go Output Generation:**
    *   Run the new Go binary on the same repo.
    *   Save output to `test/goldens/go_output.jsonl`.
3.  **Comparison Script:**
    *   Create `scripts/verify_parity.py`.
    *   Logic:
        *   Load both JSONL files.
        *   Normalize (sort keys, ignore timestamps).
        *   Assert: Every node in Legacy exists in Go.
        *   Assert: Every edge in Legacy exists in Go.
    *   *Tolerance:* Go parser might be *more* accurate. Missing nodes in Go is a failure. Extra nodes in Go is a warning (potential improvement).

## 🎯 Success Criteria
1.  **Zero Regressions:** The Go parser extracts 100% of the nodes/edges that the JS parser finds for the supported languages.
2.  **Drop-in Replacement:** The JSONL output structure is identical to the current format.
3.  **Test Coverage:** All language parsers have specific unit tests with code snippets.
4.  **Full Enrichment:** Output JSONL includes vector embeddings for all Functions.
