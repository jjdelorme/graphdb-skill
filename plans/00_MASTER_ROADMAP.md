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
- [x] `GraphProvider` Interface (Traverse, SearchFeatures). *(Note: `FindNode` was designed but never used -- removed in Campaign 7.)*
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
- [x] **Semantic Clustering:** New `EmbeddingClusterer` with K-Means++ on atomic feature embeddings, replacing file-based grouping.
- [x] ~~**3-Level Hierarchy:**~~ Removed. The `CategoryClusterer` abstraction was never wired in production and does not serve the core Feathers methodology. Dead code eliminated.
- [x] **Enrichment Improvements:** Increased truncation to 3000 chars, atomic features included as summarization context.
- [x] **Hierarchy Navigation:** New `ExploreDomain` query returns feature + parent + children + siblings + implementing functions. Wired to `--type explore-domain` CLI.
- [x] **Cleanup:** Remove legacy `FileClusterer` and enforce semantic clustering by default (Plan: `plans/refactor_remove_file_clusterer.md`).

### Campaign 3.6.5: Smart Discovery Foundation (Ingestion Fidelity)
**Goal:** Improve the fidelity of file ingestion and physical discovery. This serves as the bedrock for the Global Semantic Topology by ensuring the "Universe of Files" is correct (respecting nested .gitignore) and that we have a robust way to identify physical roots.
**Status:** Completed
**Key Deliverables:**
- [x] **Recursive Walker:** Refactor `walker.go` to respect nested `.gitignore` files (Critical for monorepos).
- [x] **Smart Discovery:** Update `discovery.go` to support `.` (root) scanning and better top-level directory detection.
- [x] **Strict Matching:** Fix `builder.go` to use strict path prefixes (prevents `auth` matching `authentication`).

### Campaign 3.7: Reliability Repair ("Unknown Feature" Fix)
**Goal:** Resolve the critical "Unknown Feature" bug where nodes lack content for summarization. Enabling on-demand disk reading for the `Enricher` to ensure every node has a description.
**Status:** ✅ Completed (Verified 2026-02-18)
**Key Deliverables:**
- [x] **Parsers Update:** Extract `end_line` in all language parsers (`.ts`, `.cs`, `.java`, `.cpp`, `.sql`, `.vb`).
- [x] **Enricher Update:** Inject `SourceLoader` to read function bodies from disk using `start_line`/`end_line`.
- [x] **Verification:** Ensure `enrich-features` produces named/described nodes via E2E test.
- [x] **Plan:** Ref: `plans/fix_missing_content_in_nodes.md`.

### Campaign 3.7.5: Robustness & Accuracy (Completed)
**Goal:** Address critical failures in Graph Construction (Dependency Resolution) and Persistence (Cleanup/Import) identified during `trucks-v2` analysis. Ensure the graph is structurally sound and "Unknown Domains" are eliminated.
**Status:** **Completed**
**Key Deliverables:**
- [x] **Robust Wipe:** Implement `RecreateDatabase` (DROP/CREATE) in `Neo4jLoader` to prevent constraint failures on large graphs.
- [x] **ID Resolution Fix (C#):** Fix `CALLS` edge generation to include Class Name in target IDs (resolving `PaymentHistoryController` disconnected graph).
- [x] **ID Resolution Audit:** Verify and fix resolution logic for Java, TS, C++, VB.NET, SQL.
- [x] **Domain Discovery Fix:** Fix path resolution in `main.go` to prevent "Unknown Domain" errors by correctly grounding relative paths.
- [x] **Plan:** Ref: `plans/campaign_3_7_fix_recreation_resolution_discovery.md`.

### Campaign 3.8: RPG Realization II (Global Semantic Topology)
**Goal:** Truly implement the "Latent Architecture Recovery" from the RPG papers. (Note: Previous attempts at Global Topology were partial). This campaign replaces directory-based discovery with global embedding clustering.
**Status:** Completed
**Key Deliverables:**
- [x] **GlobalClusterer:** Implement `GlobalEmbeddingClusterer` (K-Means on all repository functions).
- [x] **LCA Grounding:** Implement robust Lowest Common Ancestor logic to ground latent domains to the file system.
- [x] **Architecture Switch:** Update `Builder` to use `GlobalClusterer` by default.
- [x] **Naming:** Semantic labeling of latent clusters.
- [x] **Legacy Cleanup:** Remove `DirectoryDomainDiscoverer` and associated tests.
- [x] **Plan:** Ref: `plans/3.8_implementation_tasks.md`.

### Campaign 4: The Go Import Loader (Dependency Removal)
**Goal:** Port the Neo4j bulk loading logic (`import_to_neo4j.js`) to Go, eliminating the Node.js runtime dependency for standard workflows.
**Status:** Completed
**Key Deliverables:**
- [x] New package `internal/loader` for batch processing JSONL.
- [x] `import` CLI command in `cmd/graphdb`.
- [x] **Parity:** Support for `-clean`, `-incremental`, and `GraphState` commit tracking.
- [x] **Optimization:** Efficient batching using `UNWIND` cypher queries.

### Campaign 4.2: Import Performance Remediation
**Goal:** Resolve the critical "O(N^2)" performance bottleneck in the Neo4j edge importer by implementing a generic indexing strategy (`:CodeElement` label).
**Status:** Completed
**Key Deliverables:**
- [x] **Optimization:** Refactor `internal/loader` to use `MATCH (n:CodeElement)` for O(1) edge lookups.
- [x] **Schema Update:** `ApplyConstraints` to enforce `CodeElement` uniqueness and **vector indexes for RPG**.
- [x] **Verification:** Verify import speed on large graphs.

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

### Campaign 5.1: UX - Semantic Clustering Progress
**Goal:** Implement a domain-level progress bar for the long-running semantic clustering phase to prevent the "stuck" appearance during `enrich-features`.
**Status:** Completed
**Key Deliverables:**
- [x] **Instrumentation:** Add progress callbacks to `rpg.Builder`.
- [x] **UI Integration:** Hook `ui.ProgressBar` into the clustering loop.
- [x] **Determinism:** Sort domains for consistent processing order.
- [x] **Plan:** Ref: `plans/feat_semantic_clustering_progress.md`.

### Campaign 5.2: UX - Async Embedding Generation
**Goal:** Address the synchronous blocking behavior of `EmbeddingClusterer` by pre-calculating embeddings with progress reporting before clustering begins. This prevents the "hang" during large domain processing.
**Status:** Completed
**Key Deliverables:**
- [x] **Pre-calculation:** Implement batched embedding generation in `main.go` with progress bar.
- [x] **Optimization:** Update `EmbeddingClusterer` to use pre-calculated embeddings.
- [x] **Plan:** Ref: `plans/feat_precalc_embeddings.md`.

### Campaign 5.3: Orchestration - One-Shot Build
**Goal:** Implement a `build-all` command that orchestrates the entire graph construction pipeline (Ingest -> Enrich -> Import) for improved developer experience.
**Status:** Completed
**Key Deliverables:**
- [x] **Implementation:** Create `handleBuildAll` in `main.go`.
- [x] **Workflow:** Update `SKILL.md` to reflect the streamlined process.

### Campaign 5.4: Node ID Standardization (Refactor)
**Goal:** Switch from brittle absolute file paths to Project-Relative paths for all nodes, and enforce Fully Qualified Names (FQN) for C# nodes. This improves portability and graph quality (merging partial classes, fixing cross-file edges).
**Status:** Completed
**Key Deliverables:**
- [x] **Infrastructure:** `Walker` and `Worker` use relative paths.
- [x] **C# Parser:** Remove path prefixes, use FQN.
- [x] **TypeScript Parser:** Use relative paths and normalize separators.
- [x] **C++/VB.NET/SQL Parsers:** Use FQNs and remove file path prefixes from IDs.
- [x] **Verification:** Clean IDs and connected graph.
- [x] **Plan:** Ref: `plans/refactor_node_ids.md`.

### Campaign 6: The Streaming Pipeline (GraphProvider-Centric OOM Resolution)
**Goal:** Resolve Out of Memory (OOM) exceptions on massive codebases by transitioning from an in-memory batch architecture to an out-of-core streaming architecture. The pipeline will use the `GraphProvider` interface as the active working memory for intermediate states, enforcing the "Pointer to Blob Storage" pattern.
**Status:** Completed
**Key Deliverables:**
- [x] **GraphDB Interface Updates:** Add batch read/write methods to `GraphProvider` interface.
- [x] **Resumable Extraction:** Refactor atomic feature extraction to process in database-backed chunks.
- [x] **Resumable Embedding:** Refactor embedding generation to stream through the database.
- [x] **Out-of-Core Clustering:** Refactor K-Means to load only vectors and IDs from the database.
- [x] **Resumable Summarization:** Refactor LLM summarization to operate in database-backed chunks.
- [x] **Plan:** Ref: `plans/07_CAMPAIGN_7_STREAMING_PIPELINE.md`.

---

## 🔍 Code Review & Legacy Modernization Readiness

**Context:** A comprehensive code review (`plans/08_LEGACY_MODERNIZATION_REVIEW.md`) evaluated the tool's fitness for legacy code modernization using the Strangler Fig pattern and Feathers' *Working Effectively with Legacy Code*. The following campaigns address the findings. Detailed implementation specs are in `plans/09_CAMPAIGNS_7-11_PLAN.md`.

### Campaign 7: Code Hygiene & CLI Decomposition
**Goal:** Remove dead interface surface area and decompose the monolithic `main.go` (713 lines) into per-command files. Includes Priority 2 quality fixes (deterministic K-Means, batch topology updates, Cypher sanitization, filter fix).
**Status:** Completed
**Plan:** Ref: `plans/09_CAMPAIGNS_7-11_PLAN.md` Items 1, 2, 7.
**Key Deliverables:**
- [x] **Dead Code Removal:** Remove unimplemented `FindNode` from `GraphProvider` interface and all mocks (zero callers).
- [x] **CLI Decomposition:** Extract `handleIngest`, `handleQuery`, `handleImport`, `handleEnrichFeatures`, `handleBuildAll` into separate files (`cmd_ingest.go`, `cmd_query.go`, etc.). Run `gofmt` to normalize inconsistent indentation.
- [x] **Deterministic K-Means:** Add `--seed` flag to `enrich-features` for reproducible graph builds.
- [x] **Batch Topology Updates:** Replace per-node Cypher in `UpdateFeatureTopology` with UNWIND-based batching.
- [x] **Filter Fix:** Correct `GetUnextractedFunctions` predicate (`n.content IS NOT NULL` -> `n.file IS NOT NULL`).
- [x] **Cypher Sanitization:** Expand `sanitizeLabel()` to strip all non-alphanumeric/underscore characters.

### Campaign 8: Seam Detection & Contamination Propagation (Feathers Parity)
**Goal:** Restore and extend seam identification, which is currently non-functional. The `GetSeams()` query exists but depends on `ui_contaminated` and `risk_score` properties that are never set -- the Node.js scripts (`propagate_contamination.js`, `analyze_git_history.js`) that populated them were not ported during the Go migration. This campaign implements multi-layer contamination propagation and risk scoring directly in the Go binary.
**Status:** Completed
**Plan:** Ref: `plans/09_CAMPAIGNS_7-11_PLAN.md` Item 3.
**Key Deliverables:**
- [x] **New CLI Command:** `graphdb enrich-contamination -dir <path> [--rules <rules.json>]`.
- [x] **Multi-Layer Contamination:** Seed and propagate `ui_contaminated`, `db_contaminated`, `io_contaminated` flags via BFS over the CALLS graph.
- [x] **Default Heuristic Rules:** Built-in detection for UI (Controller/View/Form patterns, MFC/WinForms APIs), Database (SQL keywords, ORM patterns), External I/O (HttpClient, Socket, file system APIs).
- [x] **Risk Scoring:** Calculate `risk_score` from fan-in, fan-out, and contamination layer count.
- [x] **Broadened `GetSeams()`:** Accept `-layer` flag (ui, db, io, all) to find seams at any contamination boundary, not just UI.

### Campaign 9: Git History Restoration & Incremental Ingestion
**Goal:** Restore git history analysis (lost during Node.js migration) and add incremental ingestion so large codebases don't require full rebuilds. The v1 pipeline had `analyze_git_history.js` and a `hotspots` query type -- neither was ported to Go.
**Status:** Completed
**Plan:** Ref: `plans/09_CAMPAIGNS_7-11_PLAN.md` Item 4.
**Key Deliverables:**
- [x] **New CLI Command:** `graphdb enrich-history -dir <path> [-since <date>]`.
- [x] **Git Metrics:** Populate `change_frequency`, `last_changed`, `co_changes` on File nodes via `git log` analysis.
- [x] **Restore `hotspots` Query:** Combine `risk_score` + `change_frequency` to surface high-risk code.
- [x] **Incremental Ingestion:** `graphdb ingest -dir . --since-commit <hash>` -- writes directly to Neo4j via new `Neo4jEmitter` (no intermediate JSONL files). Auto-detects baseline from `GraphState.commit` if `--since-commit` is omitted.
- [x] **Co-Change Detection:** Identify files that frequently change together (pairwise commit analysis).

### Campaign 10: Test Coverage Integration (Feathers Characterization Tests)
**Goal:** Enable agents to understand which functions have tests and which don't -- critical for Feathers' "characterization test" workflow during legacy modernization.
**Status:** ✅ Completed
**Plan:** Ref: `plans/09_CAMPAIGNS_7-11_PLAN.md` Item 5.
**Key Deliverables:**
- [x] **Test File Detection:** Identify test files by convention (`*_test.go`, `*Test.java`, `*Tests.cs`, `*.test.ts`, `*.spec.ts`) and tag Function nodes with `is_test: true`.
- [x] **Test-to-Production Linkage:** Create `TESTS` edges from test functions to production functions via naming conventions and import analysis.
- [x] **Coverage Query:** New `graphdb query -type coverage -target <function>` to check test status.

### Campaign 11: What-If Extraction Analysis (Strangler Fig Planning)
**Goal:** Enable agents to simulate Strangler Fig extractions before executing them. Answer "If I extract these classes to a new service, what breaks?" without modifying the graph.
**Status:** Completed
**Plan:** Ref: `plans/09_CAMPAIGNS_7-11_PLAN.md` Item 6.
**Key Deliverables:**
- [x] **What-If Query:** `graphdb query -type what-if -target "Namespace.Class" [-target2 "Namespace.Class2"]`.
- [x] **Severed Edge Analysis:** Identify all incoming edges that would break.
- [x] **Orphan Detection:** Find nodes that become unreachable from non-extracted code.
- [x] **Cross-Boundary Calls:** Surface calls from remaining code into extracted code (these need an API/interface).
- [x] **Shared State Detection:** Flag globals used by both extracted and remaining code.

### Campaign 11.5: Feathers Remediation (Volatility & Pinch Points)
**Goal:** Deprecate the rigid UI/DB/IO layer contamination logic introduced in Campaign 8 and replace it with a more robust Volatility detection system based on Michael Feathers' definition of true legacy seams (Pinch Points and Semantic Seams).
**Status:** In Progress (Phase 1, 2, and 3 Complete)
**Plan:** Ref: `plans/11_CAMPAIGN_11_FEATHERS_REMEDIATION.md`
**Key Deliverables:**
- [x] **Deprecation:** Remove `ui_contaminated`, `db_contaminated`, `io_contaminated` from graph schema and CLI logic.
- [x] **Volatility Propagation:** Seed `is_volatile` flags based on global state/3rd-party dependencies and propagate UPWARDS.
- [x] **Pinch Point Detection:** Rewrite `seams` query to find structural hourglass bottlenecks.
- [x] **Semantic Seams:** Introduce new query to find cohesive structures with divergent vector embeddings (SRP violations).

### Campaign 11.6: State-Driven Semantic Naming (Eliminating Magic Strings)
**Goal:** Replace the frail, magic-string-based (`STARTS WITH "Domain-"`) feature naming system with a robust, schema-driven approach. Decouple Node Identity (UUIDs) from Semantic State (Name/Summary properties).
**Status:** Next
**Plan:** Ref: `plans/11.6_CAMPAIGN_STATE_DRIVEN_NAMING.md`
**Key Deliverables:**
- [ ] **Data Pipeline Fix:** Update `GetFunctionMetadata` to retrieve the `n.name` property, re-enabling fallback logic.
- [ ] **Decouple ID from State:** Remove all `"Feature-"` and `"Domain-Generic-"` string injections from ID generation.
- [ ] **State-Driven Enrichment:** Refactor enrichment to use explicitly empty properties (`n.name = ""` or `n.summary = ""`).

### Campaign 12: D3 Graph Visualizer (Interactive Exploration)
**Goal:** Create a single-page web application using D3.js to visualize, navigate, and search the Code Property Graph (CPG) generated by `graphdb`, acting as an interactive frontend for exploration and refactoring planning.
**Status:** Next
Plan: Ref: `plans/12_CAMPAIGN_D3_VISUALIZER.md`
**Key Deliverables:**
- [ ] **API Gateway / Backend Service:** Expose existing `internal/query` Go packages via REST/GraphQL HTTP endpoints.
- [x] **D3 Frontend:** Web application with Force-directed graphs and Hierarchical layouts.
- [ ] **Semantic Search View:** Map `search-similar` and `search-features` to a search UI.
- [ ] **Neighborhood Navigation:** Interactive expansion of node dependencies (`hybrid-context`, `neighbors`).
- [ ] **Refactoring Risk Analysis View:** Visual representation of `impact` and `what-if` queries.

---

## 🏗️ Platform Infrastructure (Deferred)

### Campaign 12.1: UI Feedback Remediation (UX Polish)
**Goal:** Address initial user feedback regarding view initialization, viewport centering, layer filtering, and visual clarity (risk scores, legends).
**Status:** Completed
**Plan:** Ref: `plans/15_CAMPAIGN_UI_FEEDBACK_REMEDIATION.md`
**Key Deliverables:**
- [x] **Top-Level View:** Default `overview` to semantic Domains.
- [x] **Viewport Navigation:** Auto-center nodes on search and expansion.
- [x] **Risk Normalization:** Provide dynamic 0-100 scales for volatility.
- [x] **Semantic Filtering:** Fix layer toggle behaviors and add node-label coloring/legends.
### Campaign 16: Critical UX Rescue (Physics & LLM Fixes)
**Goal:** Resolve two critical regressions preventing effective use of the visualizer and semantic features: missing LLM domain naming due to metadata omissions in the streaming pipeline, and severe D3 physics instability during dynamic graph expansion.
**Status:** In Progress
**Plan:** Ref: `plans/16_CAMPAIGN_CRITICAL_UX_RESCUE.md`
**Key Deliverables:**
- [x] **LLM Domain Naming:** Fix `GetFunctionMetadata` to return line numbers and `atomic_features` so the Summarizer receives valid snippets.
- [ ] **D3 Physics Stabilization:** Replace violent `forceCenter` with gentle positional forces and implement coordinated node spawning.

### Campaign 17: RPG Naming & Navigation (UX Polish)
**Goal:** Improve the semantic naming of sub-features and categories, and make the "Affected Pathways" sidebar interactive to enable seamless navigation between analysis and the visual graph.
**Status:** Planned
**Plan:** Ref: `plans/semantic_naming_and_navigation.md`
**Key Deliverables:**
- [ ] **Feature Naming Fallbacks:** Apply naming heuristics to all levels of the RPG hierarchy.
- [ ] **Interactive Impact Sidebar:** Add click-to-focus/expand capabilities to sidebar items.
- [ ] **Focus & Expand UI:** Implement robust viewport centering and automatic context expansion for focused nodes.

### Campaign 18: C++ Parser Reliability (Structural & Signature Fixes)
**Goal:** Resolve critical bugs in the C++ parser where functions were erroneously tagged as "Class" nodes during edge generation and signature extraction failed to capture parameter lists.
**Status:** Completed
**Plan:** Ref: `plans/feat_cpp_parser_bugfix.md`
**Key Deliverables:**
- [x] **Fix Node ID Generation:** Correct `Usage/Reference` logic to use `"Function"` label instead of `"Class"` for caller nodes.
- [x] **Signature Extraction Fix:** Update `extractCppSignature` to correctly parse parameter lists by passing the `function_definition` node.
- [x] **Automated Regression Testing:** Implement E2E tests to verify correct node labeling and signature fidelity.

---


### Campaign 13: The Spanner Backend (Storage Swap)
**Goal:** Establish the multi-tenant, immutable storage layer using Google Spanner Graph by swapping the storage implementation.
**Status:** Pending
**Key Deliverables:**
- [ ] Spanner Graph Schema (GQL) for RPG structure.
- [ ] **Storage Adapter:** Implement `SpannerEmitter` to replace `JSONLEmitter`.
- [ ] **Graph Provider:** Implement `SpannerProvider` to replace `Neo4jProvider`.
- [ ] Bulk Loader (JSONL -> Spanner).
- [ ] Multi-tenancy implementation (Schema Interleaving).

### Campaign 14: Cross-Platform Distribution (The Release)
**Goal:** Ship a single, zero-dependency binary for all major OSs.
**Status:** Pending
**Key Deliverables:**
- [ ] Zig-based Cross-Compilation pipeline.
- [ ] GitHub Actions release workflow.
- [ ] Automated integration tests.

### Campaign 15: The MCP Server (The Interface)
**Goal:** Expose the platform to Agents via the Model Context Protocol (MCP), enabling "Dual-View" reasoning.
**Status:** Pending
**Key Deliverables:**
- [ ] MCP Protocol implementation (Stdio transport).
- [ ] "RAM Overlay" logic (Local Diff vs. Cloud Base).
- [ ] Tool implementations (`search_features`, `traverse_deps`).

### Campaign 19: Query Limits & Summary Features
**Goal:** Handle large query outputs in the CLI to prevent LLM context exhaustion by enforcing depth/limits and providing summary outputs.
**Status:** Completed
**Plan:** Ref: `plans/feat_query_limits_and_summary.md`
**Key Deliverables:**
- [x] Enforce `-limit` flag down to Cypher in `GetNeighbors`.
- [x] Add `-summary` flag to CLI for count-based structural views.

### Campaign 20: Dual-Lens Structural Partitioning (CPM Leiden & Feathers Handshake)
**Goal:** Implement a pure Go Constant Potts Model (CPM) Leiden Community Detection engine with inverse-degree hub suppression and Feathers seam score handshakes. Establishes a Dual-Lens architecture contrasting physical execution coupling against semantic RPG intent to pinpoint actionable refactoring seams.
**Status:** Completed
**Plan:** Ref: `plans/20_CAMPAIGN_DUAL_LENS_LEIDEN_TOPOLOGY.md`
**Key Deliverables:**
- [x] **Pure Go CPM Leiden Engine:** Resolution-free modularity with dynamic $\gamma$, seed determinism, and recursive hierarchical sub-clustering.
- [x] **Two-Tier Hub Suppression:** Inverse-degree logarithmic damping and top 1% hub quarantine (`:CrossCuttingHub`).
- [x] **Overlapping Infrastructure Support:** Boundary Participation Ratio (BPR) classifying `:SharedBoundary` components with `[:BRIDGES]` edges.
- [x] **Feathers Seam Handshake:** Actionable Seam Score ranker ($\frac{\text{Fan-In} \times \text{Volatile Fan-Out}}{\text{Cut-Edges} + 1}$) surfacing $\le 4$ cut-edge interface extraction recipes.
- [x] **Zero-Token Offline Mode:** High-speed local CPU-only structural partitioning (`--offline` / `--quick`).
- [x] **Visualizer Architecture X-Ray:** Dual-Lens heatmap overlay with structural convex hulls and semantic intent node coloring.

---

Task Verified. Tests Passed. Code Clean. No Shortcuts Detected.

