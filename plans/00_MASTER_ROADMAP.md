# ARIP Master Roadmap: Repository Intelligence Platform

**Status:** Active
**Vision:** Evolve the `graphdb` skill from a local Node.js script into a scalable, multi-tenant Go + Spanner platform (ARIP).

## 🌍 Strategic Campaigns

### Campaign 1: The Go Ingestor (Language Parity)
**Goal:** Replace the Node.js extraction logic with a high-performance Go binary capable of parallel parsing and embedding, with **strict parity for existing languages**.
**Status:** Completed
**Key Deliverables:**
- [x] Standalone Go CLI (`graphdb`).
- [x] Parallel file walker with Worker Pools.
- [x] **Language Parity:** C#, C/C++, VB.NET, SQL, TypeScript, Java.
- [x] Tree-sitter integration via CGO.
- [x] **Vertex AI Integration:** Embedding generation parity with `enrich_vectors.js`.
- [x] **Data Parity:** JSONL output strictly matches existing schema (Nodes/Edges).
- [x] Standardized JSONL output format via `Storage/Emitter` interface.

### Campaign 2: The Graph Query Engine (Full Query Parity)
**Goal:** Implement the "Read" side of the platform in Go, mirroring the "Write" side (Ingestor). This enables the Go binary to answer queries directly, preparing for the Spanner migration.
**Status:** Completed
**Key Deliverables:**
- [x] `GraphProvider` Interface (FindNode, Traverse, SearchFeatures).
- [x] `Neo4jProvider` implementation (connects to local Neo4j).
- [x] **Full Query Parity:** Port all critical query types: `hybrid-context`, `test-context`, `impact`, `globals`, `suggest-seams`.
- [x] Cypher Query Builder/Manager in Go.

### Campaign 3: The RPG Core (Intent Layer Structure)
**Goal:** Define the Repository Planning Graph (RPG) schema and interfaces. (Note: Initial implementation was a skeleton/prototype).
**Status:** Completed
**Key Deliverables:**
- [x] **Schema Upgrade:** `Feature` struct and graph schema (Nodes/Edges) in Go.
- [x] **Interfaces:** `DomainDiscoverer`, `Clusterer`, `Summarizer` interfaces defined.
- [x] **CLI Commands:** `enrich-features` and `search-features` commands registered.
- [x] **Search Capability:** `search-features` query type.

### Campaign 3.5: RPG Realization (From Prototype to Production)
**Goal:** Replace the RPG placeholders (Campaign 3 skeleton) with functional logic for Domain Discovery, Clustering, Persistence, and LLM Integration. (Note: Clustering was file-based, not semantic. See Campaign 3.6 for remediation.)
**Status:** Completed
**Key Deliverables:**
- [x] **Real Domain Discovery:** Replace `SimpleDomainDiscoverer` with directory/heuristic-based logic.
- [x] **File-Based Clustering:** Replace `SimpleClusterer` with `FileClusterer` (structural grouping by filename).
- [x] **Persistence Wiring:** Ensure `enrich-features` emits `IMPLEMENTS` edges and `Feature` nodes to storage (Neo4j/JSONL).
- [x] **LLM Integration:** Connect `Summarizer` to real Vertex AI client for generation.
- [x] **E2E Verification:** Verify a real graph is built and queryable.

### Campaign 3.6: RPG Remediation (Semantic Pipeline)
**Goal:** Fix structural bugs and implement the core RPG pipeline per the research papers (RPG.pdf, RPG-Encoder.pdf): per-function atomic feature extraction, embedding-based semantic clustering, and hierarchy navigation. Gap analysis documented in `plans/rpg_gap_analysis_and_remediation.md`.
**Status:** Completed
**Key Deliverables:**
- [x] **Bug Fixes:** Corrected `IMPLEMENTS` edge direction (Function -> Feature), fixed enrichment to cover all features with domain-scoped functions, populated `ScopePath` on child features, removed dead code.
- [x] **Feature Embeddings:** `Enricher` now generates embeddings for all Feature nodes via `Embedder` integration.
- [x] **Atomic Feature Extraction:** New `FeatureExtractor` interface and `LLMFeatureExtractor` -- extracts Verb-Object descriptors per function (e.g., "validate email", "hash password").
- [x] **Semantic Clustering:** New `EmbeddingClusterer` with K-Means++ on atomic feature embeddings, replacing file-based grouping. Available via `--cluster-mode=semantic` flag.
- [x] **3-Level Hierarchy:** `Builder` supports optional `CategoryClusterer` for Domain -> Category -> Feature hierarchy (per research).
- [x] **Enrichment Improvements:** Increased truncation to 3000 chars, atomic features included as summarization context.
- [x] **Hierarchy Navigation:** New `ExploreDomain` query returns feature + parent + children + siblings + implementing functions. Wired to `--type explore-domain` CLI.

### Campaign 4: The Go Import Loader (Dependency Removal)
**Goal:** Port the Neo4j bulk loading logic (`import_to_neo4j.js`) to Go, eliminating the Node.js runtime dependency for standard workflows.
**Status:** Completed
**Key Deliverables:**
- [x] New package `internal/loader` for batch processing JSONL.
- [x] `import` CLI command in `cmd/graphdb`.
- [x] **Parity:** Support for `-clean`, `-incremental`, and `GraphState` commit tracking.
- [x] **Optimization:** Efficient batching using `UNWIND` cypher queries.

### Campaign 4.5: Gemini CLI Skill Integration (The Agent Bridge)
**Goal:** Wrap the Go Binary in a Gemini CLI Skill to allow agents to invoke it directly for **both ingestion and querying**.
**Status:** Completed
**Key Deliverables:**
- [x] Update existing JS skill (`.gemini/skills/graphdb`) to spawn the Go binary (Shims).
- [x] **Unified Interface:** Skill delegates `extract` and `query` commands to the Go binary.
- [x] Expose CLI flags (path, depth, output format) to the agent via tool definitions.
- [x] **Dependency Cleanup:** Stripped `package.json` of heavy Node.js dependencies.
- [x] **Parity:** Full parity for `hybrid-context`, `test-context`, `impact`, `globals`, `seams`, `fetch-source`, and `locate-usage`.

### Campaign 4.6: Snippet Service Extraction (Modularization)
**Goal:** Extract file slicing and pattern matching logic from `internal/query/neo4j.go` into a dedicated `internal/tools/snippet` package to ensure parity with the legacy `SnippetService.js`.
**Status:** Completed
**Key Deliverables:**
- [x] New `internal/tools/snippet` package.
- [x] `SliceFile` and `FindPatternInScope` implementation with tests.
- [x] Refactored `FetchSource` and `LocateUsage` in `Neo4jProvider`.
- [x] **Parity:** Context-aware pattern matching.

### Campaign 4.7: Parametric Traversal (Ad-Hoc Investigation)
**Goal:** Empower agents to perform ad-hoc graph exploration without predefined query templates. This introduces a flexible `traverse` command (replacing rigid queries like `impact` or `dependencies`) that accepts dynamic filters, directions, and depths. This capability acts as a functional bridge and verify-able harness for the upcoming Spanner migration (Campaign 6), ensuring complex traversals work identically on both backends.
**Status:** **Completed**
**Key Deliverables:**
- [x] **Unified Traversal API:** A single `Traverse(start, depth, criteria)` method in `GraphProvider`.
- [x] **Dynamic CLI:** A `traverse` CLI command accepting JSON-based traversal specs (replacing hardcoded flags).
- [x] **Parity Harness:** Use this generic traverser to implement existing named queries (e.g., `impact` becomes a specific configuration of `traverse`), proving the engine's flexibility.
- [x] **Plan:** Ref: `plans/feat_parametric_traversal.md`.

### Campaign 5: Structural Integrity (The "Linking" Fix)
**Goal:** Remediation of the "File-Local" linking bug found in all parsers (Java, C#, C++, TS). Currently, parsers assume dependencies exist in the caller's file, breaking the graph. We must implement Import Parsing and Symbol Resolution to enable cross-file edges.
**Status:** **Completed**
**Key Deliverables:**
- [x] **Java:** Import parsing & Type Resolution.
- [x] **Systemic:** Apply resolution logic to C#, C++, TypeScript.
- [x] **Plan:** Ref: `plans/feat_systemic_dependency_resolution.md`.
- [x] **Validation:** Verify "Impact Analysis" actually traverses files.

### Campaign 6: The Spanner Backend (Storage Swap)
**Goal:** Establish the multi-tenant, immutable storage layer using Google Spanner Graph by swapping the storage implementation.
**Status:** Pending
**Key Deliverables:**
- [ ] Spanner Graph Schema (GQL) for RPG structure.
- [ ] **Storage Adapter:** Implement `SpannerEmitter` to replace `JSONLEmitter`.
- [ ] **Graph Provider:** Implement `SpannerProvider` to replace `Neo4jProvider`.
- [ ] Bulk Loader (JSONL -> Spanner).
- [ ] Multi-tenancy implementation (Schema Interleaving).

### Campaign 7: Cross-Platform Distribution (The Release)
**Goal:** Ship a single, zero-dependency binary for all major OSs.
**Status:** Pending
**Key Deliverables:**
- [ ] Zig-based Cross-Compilation pipeline.
- [ ] GitHub Actions release workflow.
- [ ] Automated integration tests.

### Campaign 8: The MCP Server (The Interface)
**Goal:** Expose the platform to Agents via the Model Context Protocol (MCP), enabling "Dual-View" reasoning. **(Scheduled Last)**
**Status:** Pending
**Key Deliverables:**
- [ ] MCP Protocol implementation (Stdio transport).
- [ ] "RAM Overlay" logic (Local Diff vs. Cloud Base).
- [ ] Tool implementations (`search_features`, `traverse_deps`).
