# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.8.408-beta] - 2026-06-04 [Pre-release]
### Fixed
- **Enrich:** Fixed a critical validation error (`Required field is not set. field:batch_prediction_job.display_name`) when creating batch prediction jobs by setting the `DisplayName` field in the request config.

## [1.8.406-beta] - 2026-06-04 [Pre-release]
### Added
- **Build:** Added `--batch`, `--resume`, and `--gcs-bucket` flags to `build-all` to support split-phase asynchronous feature enrichment.
- **Docs:** Documented the new split-phase build-all options in `README.md` and `SKILL.md`.

## [1.8.403-beta] - 2026-06-04 [Pre-release]
### Added
- **Enrich:** Added support for the Vertex AI Gemini Batch API during feature enrichment, enabling bulk processing of feature metadata via GCS and JSONL.
- **Config:** Added support for Gemini Batch API and GCS storage bucket configurations.
- **Tests:** Added comprehensive E2E, edge case, and adversarial tests for Batch API enrichment.

## [1.8.0] - 2026-05-18
### Added
- **Config:** Added `GRAPHDB_DIR` environment variable support to set a persistent base directory for all commands.
- **Build:** Added incremental change detection to `build-all`. The tool now performs a `git diff` to verify if any supported source files have changed since the last graph state before proceeding.
- **Import:** Added `-dir` flag support to associate commits with directories.

### Changed
- **CLI:** Standardized driver and loader initialization across all commands via new abstractions. This improves CLI testability and enables mocking of the database layer.
- **RPG:** Optimized `RunClustering` to clear the existing feature topology only after new clustering results are successfully computed.
- **RPG:** Improved `RunSummarization` to handle empty batches gracefully.
- **Terminology:** Generalized instructions to use "graph database" instead of Neo4j-specific language.
- **Docs:** Removed explicit `-dir` flag from workflow examples to rely cleanly on environment variables or current directory defaults.

### Fixed
- **Query:** Enabled directory-aware status lookup. The `status` query now correctly retrieves commit hashes associated with a specific directory (stored in `:Metadata` nodes) or global state.
- **Ingest:** Fixed incremental mode detection to properly propagate the directory context.
- **Build:** Resolved a critical production build error where the `Loader` interface was undefined in `setup_prod.go`.
- **Git:** Updated `.gitignore` to prevent tracking of test binaries (`*.test`) and coverage reports (`*.out`).

### Removed
- **Skills:** Removed the `neo4j-manager` skill; management tasks are now handled externally.
- **Tracing:** Removed Agent Execution Tracing scripts and standalone trace-viewer.html.
- **Assets:** Removed unused UI assets and deprecated design documentation.
- **Artifacts:** Cleaned up temporary binary and test artifacts.
## [1.7.0] - 2026-05-14
### Added
- **Query:** Added `-dir` flag to explicitly set the base directory for source files, enabling analysis and source fetching from outside the project root.
- **Enrich:** Added support for `-dir` flag in `enrich-features` to ensure correct file resolution during RPG extraction.
- **Config:** Added support for `NEO4J_DATABASE` environment variable to allow targeting specific Neo4j databases within a single instance.

### Changed
- **Docs:** Updated default generative model to `gemini-3.1-flash-lite` and default region to `global` in configuration documentation.

### Fixed
- **Build:** Resolved compilation errors and signature mismatches in `internal/query` and `internal/rpg`.
- **Query:** Fixed source snippet resolution for `fetch-source` and `locate-usage` when working with external directories.

## [1.6.376-beta] - 2026-05-13 [Pre-release]
### Fixed
- **Build:** Resolved compilation errors and test failures in `internal/query` and `internal/rpg` introduced in the previous beta.

## [1.6.373-beta] - 2026-05-13 [Pre-release]
### Fixed
- **Enrich:** Fixed a bug where `-dir` was ignored during the extraction phase of `enrich-features`, causing file-not-found errors when run from outside the project root.
- **Query:** Added `-dir` flag support to `fetch-source` and `locate-usage` query types to ensure correct source file resolution.

## [1.6.371-beta] - 2026-05-13 [Pre-release]
### Added
- **Config:** Added support for `NEO4J_DATABASE` environment variable to allow targeting specific Neo4j databases.

## [1.6.369-beta] - 2026-05-13 [Pre-release]
### Added
- **Query:** Added `-dir` flag to explicitly set the base directory for source files when fetching snippets from outside the project root.

## [1.6.0] - 2026-05-04
### Added
- **Query:** Added a raw Cypher query passthrough mode (`-type cypher`) for arbitrary read-only database queries.
- **Query:** Added a native Duplicate Finder command (`-type duplicates`) to globally identify similar functions using vector embeddings.
- **Query:** Enforced limits on queries and added a summary flag to prevent CLI context exhaustion.
- **UI:** Added informative tooltips to action buttons (Trace Intent Hierarchy, Simulate Extraction) for better discoverability.
- **UI:** Added a unified "search-all" query endpoint to simultaneously search across features and functions.
- **Tests:** Added comprehensive mock and integration tests for the new query interfaces.
- **Docs:** Documented the new `-summary` and `-limit` global query flags in the GraphDB skill.
- **Release:** Split the release bundle by OS to reduce download size.

### Fixed
- **UI:** Automatically fit the graph visualization to the viewport on initial load, after resets, and after searches.
- **Docs:** Explicitly defined "RPG" as the Repository Planning Graph within the `GEMINI.md` hybrid architecture overview.
- **Docs:** Clarified WSL binary selection in the GraphDB skill documentation.
- **Loader:** Optimized node import query to use the global `CodeElement` index first, drastically speeding up graph import times.

## [1.5.0] - 2026-04-27
### Added
- **LLM Concurrency**: Introduced `LLM_CONCURRENCY` to configure concurrent requests during feature extraction and summarization.
- **LLM Support**: Added support for custom LLM backends via `GENAI_BACKEND` and `GENAI_BASE_URL` configuration variables.
- **Context Injection**: Added the ability to inject application context during graph enrichment via the `-app-context` flag or `OVERVIEW.md`.

### Fixed
- **Embedding Endpoints**: Fixed an issue where Vertex AI embeddings were incorrectly routed when a custom GenAI base URL was set.
- **Build**: Fixed a build conflict caused by scratchpad scripts in the `scripts` directory during `make test`.

## [1.5.347-beta] - 2026-04-17 [Pre-release]
### Added
- **LLM Concurrency**: Introduced `LLM_CONCURRENCY` to configure concurrent requests during feature extraction and summarization.
### Fixed
- **Embedding Endpoints**: Fixed an issue where Vertex AI embeddings were incorrectly routed when a custom GenAI base URL was set.
- **Build**: Fixed a build conflict caused by scratchpad scripts in the `scripts` directory during `make test`.

## [1.5.344-beta] - 2026-04-17 [Pre-release]
### Added
- **LLM Support**: Added support for custom LLM backends via `GENAI_BACKEND` and `GENAI_BASE_URL` configuration variables.
- **Context Injection**: Added the ability to inject application context during graph enrichment via the `-app-context` flag or `OVERVIEW.md`.



## [1.4.0] - 2026-04-16
### Added
- **Web UI:** Added a dedicated "Tests" layer toggle to dynamically show or hide test components (files and functions) from the graph visualization.

### Changed
- **Web UI:** Enhanced the "Reset View" button to perform a full state clear and graph data reload instead of just resetting zoom coordinates.

## [1.3.339-beta] - 2026-04-14 [Pre-release]
### Fixed
- **RPG Pipeline:** Prevented pipeline crashes on meaningless feature data by filtering out non-semantic nodes (like tests and 'unknown' placeholders). Added structured output constraints and robust handling/jitter for transient LLM API errors (429, 500+).

### Changed
- **Documentation:** Dynamically update README instructions for the latest beta release via the release manager skill.

## [1.3.336-beta] - 2026-04-14 [Pre-release]
### Fixed
- **Python Parser:** Captured correct start/end line boundaries for AST nodes using tree-sitter definition blocks instead of just identifiers.

## [1.3.334-beta] - 2026-04-14 [Pre-release]
### Fixed
- **RPG Pipeline:** Resolved RPG feature extraction bugs (including directory context), variable-hop Cypher queries for navigation, and type assertion errors.
- **Testing:** Fixed the mock end-to-end testing environment missing Neo4j dummy credentials.
### Changed
- **Config & Docs:** Updated `.gitignore` rules for `.jsonl` and untracked binaries, and added explicit instructions for 404 model errors.

## [1.3.327-beta] - 2026-04-14 [Pre-release]
### Added
- **Python Parser:** Added Python parser support using tree-sitter, including class, function, inheritance, and import resolution extraction.

## [1.3.0] - 2026-04-14
### Added
- **Server Infrastructure:** Improved server startup feedback and diagnostic error reporting during initialization.

## [1.2.323-beta] - 2026-04-10 [Pre-release]
### Changed
- **Build Compatibility:** Updated the Linux build process to use Zig CC targeting GLIBC 2.28. This ensures the pre-compiled binary is compatible with a wider range of Linux distributions, including older versions of Ubuntu (20.04+) and Debian (10+).

## [1.2.1] - 2026-03-31
### Added
- **Error Handling:** Improved Vertex AI integration to handle 404 Not Found errors as explicit hard failures. The CLI now provides users with immediate, diagnostic feedback and halts further agent execution when configuration issues (such as incorrect regions or missing projects) are detected.

### Changed
- **Documentation:** Removed redundant system documentation from this repository, as it is now managed within the `plan-commands` orchestration framework.

## [1.2.0] - 2026-03-27
### Added
- **UI Versioning:** Added a version display to the web UI footer, polling from a new `/api/health` endpoint to ensure consistency between the binary and the interface.
- **UI UX:** Enhanced the properties pane to support multi-line, full-width display for 'Atomic Features', improving readability for complex semantic clusters.
- **Architecture:** Introduced the `internal/progress` package to decouple UI progress state from core query and loader logic, successfully resolving long-standing circular dependencies.

### Changed
- **Logging:** Centralized query logging into a new `internal/logger` package. Cypher queries are now silenced in the terminal by default (directed to `io.Discard`) to prevent interference with interactive UI elements, but can still be captured via the `--log` flag.
- **Server:** Updated the `Server` struct and constructor to track and expose the binary version.

### Fixed
- **Circular Dependencies:** Resolved import cycles in the Neo4j loader and query providers by migrating shared progress tracking state to the new isolated `internal/progress` package.

## [1.1.1] - 2026-03-27
### Fixed
- **Query Hydration:** Fixed a bug in `GetNeighbors` where target node properties were omitted from the Cypher `RETURN` clause, resulting in `Unknown` labels and `null` properties in CLI/UI output.
- **C++ Resolution:** Improved symbol resolution in the C++ parser to allow name-based fallback when exact signature matches fail, preventing node fragmentation across files.
- **RPG Extraction:** Increased the source code truncation limit from 4,000 to 60,000 characters to ensure large functions (like `Auto_Plate`) are fully analyzed for feature clustering.
- **Search Resilience:** Upgraded semantic search to function as a Hybrid Search by including a literal fallback in the Cypher query (matching exact names or prefixes) alongside vector similarity scoring. Note: The API still correctly fails hard if the embedding model is completely unreachable.

## [1.1.0] - 2026-03-26
### Added
- **Documentation:** Mapped Michael Feathers legacy modernization workflows to GraphDB and detailed RPG granularity for complex God functions.

## [0.3.297-beta] - 2026-03-21 [Pre-release]
### Added
- **RPG Enrichment:** Implemented standardized progress reporting, replacing log-based output with interactive spinners and progress bars during LLM operations.
- **Analysis:** Added comprehensive research notes and metrics on RPG clustering behaviour and performance scaling.

### Changed
- **Testing:** Isolated database-dependent internal queries into an explicit `integration` build tag, allowing standard unit tests (`make test`) to run consistently without a local Neo4j container.

### Fixed
- **C++ Parser:** Corrected function call edge tagging and signature extraction logic to fix missing relationship data in the graph.
- **Testing Dependencies:** Cleaned up unused environment variables and Testcontainers imports across the testing suite.

## [0.3.290-beta] - 2026-03-19 [Pre-release]
### Added
- **UI:** Added password visibility toggle to settings modal.
- **UI:** Added configuration modal and live database stats.

### Changed
- **UI:** Repositioned physical and semantic layer controls into an overlay panel for better UX.

### Fixed
- **Parsing:** Resolved parser line span boundary issues for C++, C#, Java, SQL, and TypeScript.
- **Orchestrator:** Implemented retry and abort logic to handle errors robustly.
- **Testing:** Fixed flakiness in integration tests and removed breaking scratchpad scripts.
- **Release Manager:** Updated beta versioning format rules.

## [0.3.0-beta.2] - 2026-03-19 [Pre-release]
### Changed
- **Database Management:** Removed `-clean` flag from `build-all` and `import` CLI commands and removed `Wipe`, `RecreateDatabase`, `DropSchema` from `neo4j_loader`.
- **Documentation:** Added `make test` target and updated documentation in `GEMINI.md`. Archived completed feature plans and audit reports.

### Fixed
- **Container Cleanup:** Fixed data wipe permissions in `delete_neo4j_container.sh` using `podman unshare rm -rf`.
## [0.3.0-beta.1] - 2026-03-19 [Pre-release]
### Added
- **RPG Extraction:** Added progress tracking and robust reporting for batch operations. Integrated `CountUnextractedFunctions` for precise sizing, UI progress bars into `RunExtraction`, and periodic progress logging in `RunEmbedding`.

## [0.2.0-beta.7] - 2026-03-18 [Pre-release]
### Added
- **CLI Commands:** Added global `--log-file` option (and `GRAPHDB_LOG_FILE` env var) for persistent debugging, dual-writing standard logs with `Lshortfile` tracing.

### Fixed
- **Ingest:** Properly decode JSON numbers as `int64` and `float64` to prevent silent type assertion failures during the node and edge ingestion processes.

## [0.2.0-beta.6] - 2026-03-18 [Pre-release]
### Fixed
- **CLI Commands:** Removed unsupported `-module` flag from the `enrich-contamination` command, fixing crashes during the `build-all` orchestrator sequence.

## [0.2.0-beta.5] - 2026-03-18 [Pre-release]
### Added
- **UI/UX Improvements:** Stabilized D3 physics and improved neighborhood expansion UX for a smoother graph interaction.
- **UI Tweaks:** Cleaned up layer toggles with matching heights, simplified text, equalized spacing, and removed the unused profile avatar.

### Changed
- **Install Instructions:** Simplified installation commands by auto-generating `.gitignore` files for bundled skills during the release process.
- **Documentation:** Updated Campaign 16 Phase 2 and 3 as completed in the implementation plan.

## [0.2.0-beta.4] - 2026-03-18 [Pre-release]
### Added
- **Fail-Fast Orchestration:** Implemented error thresholds in the RPG orchestration pipelines (Extraction and Summarization) to prevent silent cascading failures during LLM processing.
- **Robust JSON Parsing:** Introduced `ParseLLMJSON` to better handle and strip markdown syntax, extra backticks, and whitespace from unstructured LLM responses.

### Changed
- **Documentation:** Archived inactive plans and renamed design documentation to `UX_DESIGN_OVERVIEW.md`.
- **Install Instructions:** Added specific instructions to README for pre-release version installation.

### Fixed
- Addressed silent feature extraction and summarization failures by failing fast and surfacing underlying context errors.


## [0.2.0-beta.3] - 2026-03-18 [Pre-release]
### Changed
- **Domain Clustering Quality:** Improved atomic feature extraction prompts to output domain-friendly, "Object-Action" (noun-first) descriptors rather than verb-centric descriptions.
- **DDD Naming Prompts:** Split Summarizer prompts into distinct "Domain" and "Feature" levels with Domain-Driven Design (DDD) naming guidance to avoid implementation-based names.
- **Enriched Node Context:** Enriched `NodeToText` with file path and function name for stronger structural and behavioral context during embedding generation.

### Fixed
- **Schema Mismatch:** Fixed `line` vs `start_line` schema property mismatch across all parsers and consumers, completely unblocking the feature extraction pipeline.

### Documentation
- Refactored `README.md` for Gemini CLI skill focus with simplified 1-line installation commands.
- Updated Scout agent and GraphDB architecture overview documentation.

## [0.2.0-beta.2] - 2026-03-17 [Pre-release]
### Changed
- **Pipeline Architecture:** Decoupled `Embedder` from the initial ingest phase, moving all high-fidelity embedding generation to Phase 3 (`enrich-features`). This speeds up ingestion, significantly reduces `nodes.jsonl` file bloat, and solves the overwrite bug.
- **Documentation:** Updated `GRAPHDB_OVERVIEW.md` and `README.md` to reflect the 6-phase pipeline architecture in detail.

### Fixed
- Fixed E2E test failures caused by un-mocked environment variable checks for `GEMINI_GENERATIVE_MODEL`.

## [0.2.0-beta.1] - 2026-03-17 [Pre-release]
### Added
- **LLM-Driven Volatility:** Extractor now seeds volatility flags using LLM heuristics during extraction.
- **Pre-flight Checks:** New safety guards for `Seams` and `Hotspots` queries that fail fast with explicit instructions when data is missing.
- **Release Manager:** Extracted release process documentation into a new dedicated skill.
- **Automated Pre-releases:** Added support to the GitHub Action workflow to automatically publish beta releases.
